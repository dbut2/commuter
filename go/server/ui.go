package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/engine"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/web"
)

func (s *Server) render(c *gin.Context, code int, comp templ.Component) {
	c.Status(code)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(c.Request.Context(), c.Writer); err != nil {
		s.fail(c, err)
	}
}

func (s *Server) requireUser(c *gin.Context) (string, bool) {
	uid, ok := s.user(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/")
		c.Abort()
	}
	return uid, ok
}

func (s *Server) initials(ctx context.Context, userID string) string {
	name, err := s.repo.UserName(ctx, userID)
	if err != nil || name == "" {
		return "?"
	}
	parts := strings.Fields(name)
	out := ""
	for _, p := range parts[:min(len(parts), 2)] {
		out += strings.ToUpper(p[:1])
	}
	return out
}

func fmtTime(t time.Time) string { return t.Local().Format("Mon 3:04 pm") }

func (s *Server) catalog(ctx context.Context, userID string) ([]web.Provider, error) {
	provs, err := s.proc.Catalog(ctx, userID)
	if err != nil {
		return nil, err
	}
	notes := map[string]string{
		"vars":     "your variables (Variables page)",
		"geocode":  "reverse-geocoded start/end",
		"segments": "your segments (Settings page)",
	}
	out := make([]web.Provider, 0, len(provs))
	for _, p := range provs {
		vp := web.Provider{Name: p.Name(), Note: notes[p.Name()]}
		for _, f := range p.Data() {
			vf := web.Field{Key: f.Name, Kind: string(f.Type.Kind), Values: f.Type.Values, Example: f.Example}
			for _, o := range f.Type.Operators() {
				vf.Ops = append(vf.Ops, web.Op{Name: o.Name, Label: o.Label, Multi: o.Multi})
			}
			vp.Fields = append(vp.Fields, vf)
		}
		out = append(out, vp)
	}
	return out, nil
}

func (s *Server) root(c *gin.Context) {
	if _, ok := s.user(c); ok {
		c.Redirect(http.StatusSeeOther, "/activities")
		return
	}
	s.render(c, http.StatusOK, web.LoginPage())
}

func activityName(strava map[string]string, stravaID int64) string {
	if n := strava["activity_name"]; n != "" {
		return n
	}
	return fmt.Sprintf("Activity %d", stravaID)
}

func activityMeta(strava map[string]string) string {
	var parts []string
	add := func(s string) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	add(strava["activity_time"])
	add(strava["activity_type"])
	if d := strava["activity_distance"]; d != "" {
		add(d + " km")
	}
	add(strava["activity_moving_duration"])
	return strings.Join(parts, " | ")
}

func feedCard(it store.FeedItem) web.ActivityCard {
	card := web.ActivityCard{
		ID:     it.ActivityID,
		Name:   activityName(it.Strava, it.StravaID),
		Meta:   activityMeta(it.Strava),
		Status: it.Status,
		Rules:  it.AppliedRules,
	}
	if j := it.Job; j != nil {
		switch {
		case j.Status == "failed":
			card.Status = "failed"
			card.Note = j.LastError
		case it.Status == "pending" && j.Status == "queued":
			card.Note = "next poll " + fmtTime(j.NextRun)
		case it.Status == "unprocessed":
			card.Note = "queued"
		}
	}
	return card
}

func (s *Server) activitiesPage(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	feed, err := s.repo.Feed(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	d := web.Dashboard{Filter: c.Query("status")}
	for _, it := range feed {
		card := feedCard(it)
		switch card.Status {
		case "pending":
			d.PendingCount++
		case "failed":
			d.FailedCount++
		}
		if d.Filter == "" || card.Status == d.Filter {
			d.Cards = append(d.Cards, card)
		}
	}
	s.render(c, http.StatusOK, web.DashboardPage(d, s.initials(ctx, userID)))
}

func (s *Server) activitiesSync(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	if _, err := s.proc.Sync(c.Request.Context(), userID); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/activities")
}

func (s *Server) activityPage(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	info, err := s.repo.ActivityDetail(ctx, userID, c.Param("id"))
	if err != nil {
		s.fail(c, err)
		return
	}
	if info == nil {
		c.String(http.StatusNotFound, "activity not found")
		return
	}
	vm := web.ActivityPage{
		ID:       info.ActivityID,
		StravaID: info.StravaID,
		Name:     activityName(info.Strava, info.StravaID),
		Meta:     activityMeta(info.Strava),
		Status:   info.Status,
	}
	if j := info.Job; j != nil {
		vm.JobStatus = j.Status
		vm.JobError = j.LastError
		vm.NextRun = fmtTime(j.NextRun)
		vm.ExpiresAt = fmtTime(j.ExpiresAt)
		if j.Status == "failed" {
			vm.Status = "failed"
		}
	}
	for _, r := range info.RunLog {
		vm.RunLog = append(vm.RunLog, web.RunRow{Rule: r.Rule, Status: r.Status, Note: r.Note})
	}
	for _, p := range info.Providers {
		pd := web.ProviderData{Provider: p.Provider, Found: p.Found, FetchedAt: fmtTime(p.FetchedAt)}
		keys := make([]string, 0, len(p.Data))
		for k := range p.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pd.Data = append(pd.Data, [2]string{k, p.Data[k]})
		}
		vm.Providers = append(vm.Providers, pd)
	}
	s.render(c, http.StatusOK, web.ActivityDetailPage(vm, s.initials(ctx, userID)))
}

func (s *Server) activityRerun(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	info, err := s.repo.ActivityDetail(ctx, userID, c.Param("id"))
	if err != nil {
		s.fail(c, err)
		return
	}
	if info == nil {
		c.String(http.StatusNotFound, "activity not found")
		return
	}
	if err := s.proc.Enqueue(ctx, userID, info.StravaID); err != nil {
		s.fail(c, err)
		return
	}
	s.proc.KickPoll()
	c.Redirect(http.StatusSeeOther, "/activities/"+info.ActivityID)
}

func condSummary(catalog []web.Provider, conds []engine.Cond) string {
	parts := make([]string, len(conds))
	for i, cd := range conds {
		op := cd.Op
		if f, ok := web.FindField(catalog, cd.Field); ok {
			for _, o := range f.Ops {
				if o.Name == cd.Op {
					op = o.Label
				}
			}
		}
		field := cd.Field
		if i := strings.IndexByte(field, '.'); i >= 0 {
			field = field[i+1:]
		}
		parts[i] = fmt.Sprintf("%s %s %s", field, op, cd.Value)
	}
	return strings.Join(parts, ", ")
}

func actSummary(acts []engine.Act) string {
	labels := map[string]string{"title": "Set title", "desc": "Set description", "commute": "Mark commute", "hide": "Hide from home feed"}
	parts := make([]string, len(acts))
	for i, a := range acts {
		l := labels[a.Key]
		if l == "" {
			l = a.Key
		}
		parts[i] = l
	}
	return strings.Join(parts, ", ")
}

func (s *Server) rulesPage(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	rules, err := s.repo.Rules(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	catalog, err := s.catalog(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	rows := make([]web.RuleRow, len(rules))
	for i, r := range rules {
		rows[i] = web.RuleRow{
			ID: r.ID, Name: r.Name, Enabled: r.Enabled,
			When: condSummary(catalog, r.Conds),
			Then: actSummary(r.Acts),
		}
	}
	s.render(c, http.StatusOK, web.RulesPage(rows, s.initials(ctx, userID)))
}

func (s *Server) ruleToggle(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	if err := s.repo.ToggleRule(c.Request.Context(), userID, c.Query("id")); err != nil {
		s.fail(c, err)
		return
	}
	if c.GetHeader("HX-Request") != "" {
		c.Status(http.StatusNoContent)
		return
	}
	c.Redirect(http.StatusSeeOther, "/rules")
}

func (s *Server) ruleMove(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	err := s.repo.MoveRule(c.Request.Context(), userID, c.PostForm("id"), c.PostForm("dir") == "up")
	if err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/rules")
}

func (s *Server) ruleEditor(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	catalog, err := s.catalog(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	vm := web.Editor{Enabled: true, Catalog: catalog}
	if id := c.Query("id"); id != "" {
		r, err := s.repo.Rule(ctx, userID, id)
		if err != nil {
			s.fail(c, err)
			return
		}
		if r == nil {
			c.String(http.StatusNotFound, "rule not found")
			return
		}
		vm.ID, vm.Name, vm.Enabled = r.ID, r.Name, r.Enabled
		for _, cd := range r.Conds {
			vm.Conds = append(vm.Conds, web.CondRowVM(cd))
		}
		for _, a := range r.Acts {
			vm.Acts = append(vm.Acts, web.ActRowVM{Key: a.Key, Template: a.Template})
		}
	}
	if len(vm.Conds) == 0 {
		vm.Conds = []web.CondRowVM{{Field: "strava.activity_type", Op: "eq"}}
	}
	if len(vm.Acts) == 0 {
		vm.Acts = []web.ActRowVM{{Key: "title"}}
	}
	s.render(c, http.StatusOK, web.EditorPage(vm, s.initials(ctx, userID)))
}

func parseRuleForm(c *gin.Context) (engine.Rule, []web.CondRowVM, []web.ActRowVM) {
	r := engine.Rule{
		Name:    strings.TrimSpace(c.PostForm("name")),
		Enabled: c.PostForm("enabled") != "",
	}
	fields, ops, values := c.PostFormArray("cond_field"), c.PostFormArray("cond_op"), c.PostFormArray("cond_value")
	var condVMs []web.CondRowVM
	for i := range fields {
		if i >= len(ops) || i >= len(values) {
			break
		}
		cd := engine.Cond{Field: fields[i], Op: ops[i], Value: strings.TrimSpace(values[i])}
		r.Conds = append(r.Conds, cd)
		condVMs = append(condVMs, web.CondRowVM(cd))
	}
	keys, templates := c.PostFormArray("act_key"), c.PostFormArray("act_template")
	var actVMs []web.ActRowVM
	for i := range keys {
		if i >= len(templates) {
			break
		}
		a := engine.Act{Key: keys[i], Template: templates[i]}
		r.Acts = append(r.Acts, a)
		actVMs = append(actVMs, web.ActRowVM{Key: a.Key, Template: a.Template})
	}
	return r, condVMs, actVMs
}

func validateRule(catalog []web.Provider, r engine.Rule) string {
	if r.Name == "" {
		return "Rule needs a name."
	}
	if len(r.Acts) == 0 {
		return "Rule needs at least one action."
	}
	for _, cd := range r.Conds {
		f, ok := web.FindField(catalog, cd.Field)
		if !ok {
			return fmt.Sprintf("Unknown field %q.", cd.Field)
		}
		opOK := false
		for _, o := range f.Ops {
			opOK = opOK || o.Name == cd.Op
		}
		if !opOK {
			return fmt.Sprintf("Field %q has no operator %q.", cd.Field, cd.Op)
		}
		if !strings.Contains(cd.Value, "{") {
			if _, err := core.ParseValue(core.Kind(f.Kind), cd.Value); err != nil && f.Kind != string(core.KindCoords) {
				return fmt.Sprintf("Bad value for %s: %q.", cd.Field, cd.Value)
			}
			if f.Kind == string(core.KindCoords) {
				if _, err := core.ParseNearTarget(cd.Value); err != nil {
					return fmt.Sprintf("Bad coords value %q — want \"lat, lon\" with an optional radius like \"300m\".", cd.Value)
				}
			}
		}
	}
	return ""
}

func (s *Server) ruleSave(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	catalog, err := s.catalog(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	r, condVMs, actVMs := parseRuleForm(c)
	r.ID = c.Query("id")
	if msg := validateRule(catalog, r); msg != "" {
		vm := web.Editor{ID: r.ID, Name: r.Name, Enabled: r.Enabled, Conds: condVMs, Acts: actVMs, Catalog: catalog, Error: msg}
		s.render(c, http.StatusUnprocessableEntity, web.EditorPage(vm, s.initials(ctx, userID)))
		return
	}
	if r.ID == "" {
		err = s.repo.CreateRule(ctx, userID, &r)
		if err == nil && !r.Enabled {
			err = s.repo.ToggleRule(ctx, userID, r.ID)
		}
	} else {
		err = s.repo.UpdateRule(ctx, userID, &r)
	}
	if err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/rules")
}

func (s *Server) ruleDelete(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	if err := s.repo.DeleteRule(c.Request.Context(), userID, c.Query("id")); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/rules")
}

func (s *Server) condRowFragment(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	catalog, err := s.catalog(c.Request.Context(), userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	vm := web.CondRowVM{
		Field: c.Query("cond_field"),
		Op:    c.Query("cond_op"),
		Value: c.Query("cond_value"),
	}
	if vm.Field == "" {
		vm.Field = "strava.activity_type"
	}
	if c.GetHeader("HX-Trigger-Name") == "cond_field" {
		vm.Op, vm.Value = "", ""
	}
	s.render(c, http.StatusOK, web.CondRow(catalog, vm))
}

func (s *Server) actRowFragment(c *gin.Context) {
	if _, ok := s.requireUser(c); !ok {
		return
	}
	vm := web.ActRowVM{Key: c.Query("act_key"), Template: c.Query("act_template")}
	if vm.Key == "" {
		vm.Key = "title"
	}
	s.render(c, http.StatusOK, web.ActRow(vm))
}

func (s *Server) settingsPage(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	parkrunID, err := s.repo.ParkrunID(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	segments, err := s.repo.Segments(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	rows := make([]web.SegmentRow, len(segments))
	for i, seg := range segments {
		rows[i] = web.SegmentRow{Name: seg.Name, SegmentID: strconv.FormatInt(seg.SegmentID, 10)}
	}
	s.render(c, http.StatusOK, web.SettingsPage(parkrunID, rows, c.Query("saved") != "", s.initials(ctx, userID)))
}

func (s *Server) settingsParkrun(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.PostForm("parkrun_id"))
	if id != "" {
		ok := id[0] == 'A' || id[0] == 'a' || (id[0] >= '0' && id[0] <= '9')
		for _, r := range id[1:] {
			ok = ok && r >= '0' && r <= '9'
		}
		if !ok {
			c.String(http.StatusBadRequest, "parkrun ID should look like A1234567")
			return
		}
	}
	if err := s.repo.SetParkrunID(c.Request.Context(), userID, id); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?saved=1")
}

func (s *Server) segmentSave(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	id, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("segment_id")), 10, 64)
	if !validVarName(name) || err != nil || id <= 0 {
		c.String(http.StatusBadRequest, "bad segment name or ID")
		return
	}
	if err := s.repo.SetSegment(c.Request.Context(), userID, store.Segment{Name: name, SegmentID: id}); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?saved=1")
}

func (s *Server) segmentDelete(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	if err := s.repo.DeleteSegment(c.Request.Context(), userID, c.PostForm("name")); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings")
}

func (s *Server) varsPage(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	vars, err := s.repo.Vars(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	rules, err := s.repo.Rules(ctx, userID)
	if err != nil {
		s.fail(c, err)
		return
	}
	rows := make([]web.VarRow, len(vars))
	for i, v := range vars {
		used := 0
		token := "{vars." + v.Name + "}"
		for _, r := range rules {
			hit := false
			for _, cd := range r.Conds {
				hit = hit || strings.Contains(cd.Value, token)
			}
			for _, a := range r.Acts {
				hit = hit || strings.Contains(a.Template, token)
			}
			if hit {
				used++
			}
		}
		usedLabel := "unused"
		switch used {
		case 0:
		case 1:
			usedLabel = "used by 1 rule"
		default:
			usedLabel = fmt.Sprintf("used by %d rules", used)
		}
		rows[i] = web.VarRow{Name: v.Name, Type: v.Type, Value: v.Value, Used: usedLabel}
	}
	s.render(c, http.StatusOK, web.VarsPage(rows, s.initials(ctx, userID)))
}

func validVarName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}

func (s *Server) varSave(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	v := store.Var{
		Name:  strings.TrimSpace(c.PostForm("name")),
		Type:  c.PostForm("type"),
		Value: strings.TrimSpace(c.PostForm("value")),
	}
	kindOK := false
	for _, t := range web.VarTypes {
		kindOK = kindOK || v.Type == t
	}
	if !validVarName(v.Name) || !kindOK {
		c.String(http.StatusBadRequest, "bad variable name or type")
		return
	}
	if _, err := core.ParseValue(core.Kind(v.Type), v.Value); err != nil {
		c.String(http.StatusBadRequest, "bad %s value %q", v.Type, v.Value)
		return
	}
	if err := s.repo.SetVar(c.Request.Context(), userID, v); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/vars")
}

func (s *Server) varDelete(c *gin.Context) {
	userID, ok := s.requireUser(c)
	if !ok {
		return
	}
	if err := s.repo.DeleteVar(c.Request.Context(), userID, c.PostForm("name")); err != nil {
		s.fail(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/vars")
}
