// Renderer for the kro Azure Landing Zone prototype.
//
// Reads a small declarative authoring file (landingzone.yaml), fetches one or
// more Azure-Landing-Zones-Library-shaped libraries from GitHub (ordered, with
// later libraries overriding earlier ones for the same file basename), and
// emits a complete kro ResourceGraphDefinition.
//
// The graph chains its DAG entirely through CEL refs:
//   - child ManagementGroup -> parent ManagementGroup
//   - ManagementGroupPolicyAssignment -> its ManagementGroup
//   - ManagementGroupPolicyAssignment -> custom PolicyDefinition CR (when the
//     resolved assignment targets a definition that lives in the library, not
//     a built-in GUID)
//   - PolicyDefinition -> intermediate-root ManagementGroup (scope)
//
// kro infers reconciliation order from these refs; no dependsOn is used.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------- input schema ----------

type Input struct {
	ParentResourceID           string                                `yaml:"parentResourceId"`
	Prefix                     string                                `yaml:"prefix"`
	Location                   string                                `yaml:"location"`
	Libraries                  []LibraryRef                          `yaml:"libraries"`
	BaseArchitecture           string                                `yaml:"baseArchitecture"`
	ManagementGroups           []MGOverride                          `yaml:"managementGroups"`
	Archetypes                 map[string]ArchetypeOverlay           `yaml:"archetypes"`
	ArchetypeOverrides         map[string]ArchetypeAddRemove         `yaml:"archetypeOverrides"`
	PolicyDefaultValues        map[string]string                     `yaml:"policyDefaultValues"`
	PolicyAssignmentsToModify  map[string]map[string]AssignmentMod   `yaml:"policyAssignmentsToModify"`
	PolicyAssignmentsToDisable map[string][]string                   `yaml:"policyAssignmentsToDisable"`
}

type LibraryRef struct {
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
	Path string `yaml:"path"`
}

type MGOverride struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"displayName"`
	Parent      string   `yaml:"parent"`
	Archetypes  []string `yaml:"archetypes"`
	Disabled    bool     `yaml:"disabled"`
}

type ArchetypeOverlay struct {
	PolicyAssignments    []string `yaml:"policyAssignments"`
	PolicyDefinitions    []string `yaml:"policyDefinitions"`
	PolicySetDefinitions []string `yaml:"policySetDefinitions"`
}

type ArchetypeAddRemove struct {
	Add    []string `yaml:"add"`
	Remove []string `yaml:"remove"`
}

type AssignmentMod struct {
	EnforcementMode string                 `yaml:"enforcementMode"`
	Parameters      map[string]interface{} `yaml:"parameters"`
	Identity        map[string]interface{} `yaml:"identity"`
	Location        string                 `yaml:"location"`
}

// ---------- library payloads ----------

type ArchitectureDoc struct {
	Name             string                  `json:"name"`
	ManagementGroups []ArchitectureMG        `json:"management_groups"`
}

type ArchitectureMG struct {
	ID          string   `json:"id"`
	ParentID    *string  `json:"parent_id"`
	DisplayName string   `json:"display_name"`
	Archetypes  []string `json:"archetypes"`
	Exists      bool     `json:"exists"`
}

type ArchetypeDoc struct {
	Name                 string   `json:"name"`
	PolicyAssignments    []string `json:"policy_assignments"`
	PolicyDefinitions    []string `json:"policy_definitions"`
	PolicySetDefinitions []string `json:"policy_set_definitions"`
	RoleDefinitions      []string `json:"role_definitions"`
}

type PolicyDefaultValuesDoc struct {
	Defaults []PolicyDefault `json:"defaults"`
}

type PolicyDefault struct {
	DefaultName       string                  `json:"default_name"`
	Description       string                  `json:"description"`
	PolicyAssignments []PolicyDefaultTarget   `json:"policy_assignments"`
}

type PolicyDefaultTarget struct {
	PolicyAssignmentName string   `json:"policy_assignment_name"`
	ParameterNames       []string `json:"parameter_names"`
}

// raw assignment / definition JSON kept as a generic map so we can pass it
// through to the rendered YAML without losing fields.
type rawJSON = map[string]interface{}

// ---------- merged catalogue ----------

type Catalogue struct {
	Architectures map[string]*ArchitectureDoc
	Archetypes    map[string]*ArchetypeDoc
	Assignments   map[string]rawJSON
	Definitions   map[string]rawJSON
	Defaults      *PolicyDefaultValuesDoc

	// URL maps for lazy fetch.
	assignURLs map[string]string
	defURLs    map[string]string
}

func newCatalogue() *Catalogue {
	return &Catalogue{
		Architectures: map[string]*ArchitectureDoc{},
		Archetypes:    map[string]*ArchetypeDoc{},
		Assignments:   map[string]rawJSON{},
		Definitions:   map[string]rawJSON{},
		Defaults:      &PolicyDefaultValuesDoc{},
	}
}

// ---------- library fetch ----------

type ghTree struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
	Truncated bool `json:"truncated"`
}

func httpGetJSON(url string, out interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %d: %s", url, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func httpGetBytes(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: %d: %s", url, resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// fetchLibrary lists every relevant file under path/ for the given library
// reference, returning maps keyed by file basename (without the suffix).
type libraryFiles struct {
	architectures map[string]string // basename -> raw URL
	archetypes    map[string]string
	assignments   map[string]string
	definitions   map[string]string
	defaultsURL   string // raw URL or "" if absent
}

func fetchLibraryListing(lib LibraryRef) (*libraryFiles, error) {
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", lib.Repo, lib.Ref)
	var tree ghTree
	if err := httpGetJSON(treeURL, &tree); err != nil {
		return nil, fmt.Errorf("list library %s@%s: %w", lib.Repo, lib.Ref, err)
	}
	if tree.Truncated {
		return nil, fmt.Errorf("library %s@%s tree is truncated; refusing to render with partial data", lib.Repo, lib.Ref)
	}

	path := strings.Trim(lib.Path, "/")
	rawBase := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/", lib.Repo, lib.Ref)

	out := &libraryFiles{
		architectures: map[string]string{},
		archetypes:    map[string]string{},
		assignments:   map[string]string{},
		definitions:   map[string]string{},
	}
	for _, e := range tree.Tree {
		if e.Type != "blob" {
			continue
		}
		p := e.Path
		if path != "" && !strings.HasPrefix(p, path+"/") {
			continue
		}
		rel := strings.TrimPrefix(p, path+"/")
		switch {
		case strings.HasPrefix(rel, "architecture_definitions/") && strings.HasSuffix(rel, ".alz_architecture_definition.json"):
			base := strings.TrimSuffix(strings.TrimPrefix(rel, "architecture_definitions/"), ".alz_architecture_definition.json")
			out.architectures[base] = rawBase + p
		case strings.HasPrefix(rel, "archetype_definitions/") && strings.HasSuffix(rel, ".alz_archetype_definition.json"):
			base := strings.TrimSuffix(strings.TrimPrefix(rel, "archetype_definitions/"), ".alz_archetype_definition.json")
			out.archetypes[base] = rawBase + p
		case strings.HasPrefix(rel, "policy_assignments/") && strings.HasSuffix(rel, ".alz_policy_assignment.json"):
			base := strings.TrimSuffix(strings.TrimPrefix(rel, "policy_assignments/"), ".alz_policy_assignment.json")
			out.assignments[base] = rawBase + p
		case strings.HasPrefix(rel, "policy_definitions/") && strings.HasSuffix(rel, ".alz_policy_definition.json"):
			base := strings.TrimSuffix(strings.TrimPrefix(rel, "policy_definitions/"), ".alz_policy_definition.json")
			out.definitions[base] = rawBase + p
		case rel == "alz_policy_default_values.json":
			out.defaultsURL = rawBase + p
		}
	}
	return out, nil
}

// mergeListings merges per-library file URL maps in input order; later
// libraries override earlier ones on the same basename. Returned maps are
// indexed by basename -> raw URL of the winning entry.
func mergeListings(listings []*libraryFiles) *libraryFiles {
	out := &libraryFiles{
		architectures: map[string]string{},
		archetypes:    map[string]string{},
		assignments:   map[string]string{},
		definitions:   map[string]string{},
	}
	for _, l := range listings {
		for k, v := range l.architectures {
			out.architectures[k] = v
		}
		for k, v := range l.archetypes {
			out.archetypes[k] = v
		}
		for k, v := range l.assignments {
			out.assignments[k] = v
		}
		for k, v := range l.definitions {
			out.definitions[k] = v
		}
		if l.defaultsURL != "" {
			out.defaultsURL = l.defaultsURL // later wins; merged at JSON level below
		}
	}
	return out
}

// loadDefaults loads and merges alz_policy_default_values.json from every
// library in order. Merge unit is one entry of the `defaults` array keyed by
// `default_name`; later libraries replace same-named entries.
func loadDefaults(listings []*libraryFiles) (*PolicyDefaultValuesDoc, error) {
	merged := map[string]PolicyDefault{}
	order := []string{}
	for _, l := range listings {
		if l.defaultsURL == "" {
			continue
		}
		body, err := httpGetBytes(l.defaultsURL)
		if err != nil {
			return nil, fmt.Errorf("fetch defaults %s: %w", l.defaultsURL, err)
		}
		var doc PolicyDefaultValuesDoc
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parse defaults %s: %w", l.defaultsURL, err)
		}
		for _, d := range doc.Defaults {
			if _, ok := merged[d.DefaultName]; !ok {
				order = append(order, d.DefaultName)
			}
			merged[d.DefaultName] = d
		}
	}
	out := &PolicyDefaultValuesDoc{}
	for _, k := range order {
		out.Defaults = append(out.Defaults, merged[k])
	}
	return out, nil
}

// ---------- helpers ----------

var (
	customDefRefRE = regexp.MustCompile(`/providers/Microsoft\.Management/managementGroups/[^/]+/providers/Microsoft\.Authorization/policyDefinitions/([^/]+)$`)
	customSetRefRE = regexp.MustCompile(`/providers/Microsoft\.Management/managementGroups/[^/]+/providers/Microsoft\.Authorization/policySetDefinitions/([^/]+)$`)
	builtInDefRE   = regexp.MustCompile(`^/providers/Microsoft\.Authorization/policy(Set)?Definitions/[^/]+$`)
	defaultLocRE   = regexp.MustCompile(`\$\{default_location\}`)
)

func mustJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// camel turns a kebab/snake-style string into a CamelCase token usable as a
// kro resource id.
func camel(s string) string {
	parts := regexp.MustCompile(`[^A-Za-z0-9]+`).Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

func kebab(parts ...string) string {
	joined := strings.Join(parts, "-")
	joined = strings.ToLower(joined)
	joined = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(joined, "-")
	joined = regexp.MustCompile(`-+`).ReplaceAllString(joined, "-")
	return strings.Trim(joined, "-")
}

func mgID(name string) string     { return "mg" + camel(name) }
func defID(name string) string    { return "pd" + camel(name) }
func paID(mg, a string) string    { return "pa" + camel(mg) + camel(a) }

// ---------- composition ----------

type effectiveMG struct {
	Name        string
	DisplayName string
	Parent      string // mg name in the effective list, or "root"
	Archetypes  []string
}

func composeMGs(in *Input, cat *Catalogue) ([]effectiveMG, error) {
	mgsByName := map[string]*effectiveMG{}
	order := []string{}

	// Seed from baseArchitecture if specified.
	if in.BaseArchitecture != "" {
		arch, ok := cat.Architectures[in.BaseArchitecture]
		if !ok {
			return nil, fmt.Errorf("baseArchitecture %q not found in any library", in.BaseArchitecture)
		}
		for _, m := range arch.ManagementGroups {
			parent := "root"
			if m.ParentID != nil {
				parent = *m.ParentID
			}
			mg := &effectiveMG{
				Name:        m.ID,
				DisplayName: m.DisplayName,
				Parent:      parent,
				Archetypes:  append([]string(nil), m.Archetypes...),
			}
			mgsByName[m.ID] = mg
			order = append(order, m.ID)
		}
	}

	// Apply input overrides.
	for _, ov := range in.ManagementGroups {
		if ov.Name == "" {
			return nil, fmt.Errorf("managementGroups entry without name")
		}
		existing, ok := mgsByName[ov.Name]
		if ov.Disabled {
			if !ok {
				return nil, fmt.Errorf("managementGroups: cannot disable unknown MG %q", ov.Name)
			}
			delete(mgsByName, ov.Name)
			// remove from order
			for i, n := range order {
				if n == ov.Name {
					order = append(order[:i], order[i+1:]...)
					break
				}
			}
			continue
		}
		if !ok {
			// Adding a new MG.
			if ov.Parent == "" {
				return nil, fmt.Errorf("managementGroups: new MG %q must specify parent", ov.Name)
			}
			dn := ov.DisplayName
			if dn == "" {
				dn = ov.Name
			}
			mgsByName[ov.Name] = &effectiveMG{
				Name:        ov.Name,
				DisplayName: dn,
				Parent:      ov.Parent,
				Archetypes:  append([]string(nil), ov.Archetypes...),
			}
			order = append(order, ov.Name)
			continue
		}
		// Overriding an existing MG.
		if ov.DisplayName != "" {
			existing.DisplayName = ov.DisplayName
		}
		if ov.Parent != "" {
			existing.Parent = ov.Parent
		}
		if ov.Archetypes != nil {
			existing.Archetypes = append([]string(nil), ov.Archetypes...)
		}
	}

	// Validate parents.
	for _, name := range order {
		mg := mgsByName[name]
		if mg.Parent == "root" {
			continue
		}
		if _, ok := mgsByName[mg.Parent]; !ok {
			return nil, fmt.Errorf("MG %q references unknown parent %q", mg.Name, mg.Parent)
		}
	}

	out := make([]effectiveMG, 0, len(order))
	for _, name := range order {
		out = append(out, *mgsByName[name])
	}
	return out, nil
}

// composeArchetype returns the effective list of policy assignments for a
// single archetype name, applying archetypeOverrides and custom archetypes
// on top of the library catalogue.
func composeArchetypeAssignments(in *Input, cat *Catalogue, name string) ([]string, error) {
	var base []string
	libArch, libOk := cat.Archetypes[name]
	customArch, customOk := in.Archetypes[name]
	if libOk {
		base = append(base, libArch.PolicyAssignments...)
	}
	if customOk {
		if libOk {
			// A custom archetype with the same name as a library one replaces
			// it; keep the union semantics simple by treating custom as the
			// definitive list for that archetype.
			base = append([]string(nil), customArch.PolicyAssignments...)
		} else {
			base = append(base, customArch.PolicyAssignments...)
		}
	}
	if !libOk && !customOk {
		return nil, fmt.Errorf("archetype %q not found in library or in `archetypes` overlay", name)
	}
	if override, ok := in.ArchetypeOverrides[name]; ok {
		base = applyAddRemove(base, override.Add, override.Remove)
	}
	return dedup(base), nil
}

func applyAddRemove(base, add, remove []string) []string {
	rm := map[string]bool{}
	for _, r := range remove {
		rm[r] = true
	}
	out := base[:0:0]
	for _, b := range base {
		if !rm[b] {
			out = append(out, b)
		}
	}
	out = append(out, add...)
	return out
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := in[:0:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ---------- assignment composition ----------

// composeAssignment starts from the library assignment JSON, applies
// policyDefaultValues substitution, then applies per-(mg,assignment)
// overrides from PolicyAssignmentsToModify.
func composeAssignment(in *Input, cat *Catalogue, mg, name string) (rawJSON, error) {
	raw, ok := cat.Assignments[name]
	if !ok {
		return nil, fmt.Errorf("assignment %q (referenced by MG %q) not found in library", name, mg)
	}
	// deep-copy via JSON round-trip so per-(mg) modifications don't bleed.
	b, _ := json.Marshal(raw)
	var dup rawJSON
	_ = json.Unmarshal(b, &dup)

	props, _ := dup["properties"].(map[string]interface{})
	if props == nil {
		props = map[string]interface{}{}
		dup["properties"] = props
	}
	params, _ := props["parameters"].(map[string]interface{})
	if params == nil {
		params = map[string]interface{}{}
		props["parameters"] = params
	}

	// 1. policyDefaultValues -> assignment parameter substitution.
	if cat.Defaults != nil {
		for _, d := range cat.Defaults.Defaults {
			val, present := in.PolicyDefaultValues[d.DefaultName]
			if !present {
				continue
			}
			for _, target := range d.PolicyAssignments {
				if target.PolicyAssignmentName != name {
					continue
				}
				for _, pname := range target.ParameterNames {
					params[pname] = map[string]interface{}{"value": val}
				}
			}
		}
	}

	// 2. policyAssignmentsToModify overrides.
	if perMG, ok := in.PolicyAssignmentsToModify[mg]; ok {
		if mod, ok := perMG[name]; ok {
			if mod.EnforcementMode != "" {
				props["enforcementMode"] = mod.EnforcementMode
			}
			for k, v := range mod.Parameters {
				params[k] = map[string]interface{}{"value": v}
			}
			if mod.Identity != nil {
				dup["identity"] = mod.Identity
			}
			if mod.Location != "" {
				dup["location"] = mod.Location
			}
		}
	}

	return dup, nil
}

// ---------- ordered YAML emission ----------

func n(value interface{}) *yaml.Node {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		panic(err)
	}
	return &node
}

func mapping(pairs ...interface{}) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(pairs); i += 2 {
		k := pairs[i].(string)
		v := pairs[i+1]
		var vn *yaml.Node
		switch x := v.(type) {
		case *yaml.Node:
			vn = x
		default:
			vn = n(x)
		}
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, vn)
	}
	return m
}

func seq(items ...*yaml.Node) *yaml.Node {
	s := &yaml.Node{Kind: yaml.SequenceNode}
	s.Content = append(s.Content, items...)
	return s
}

func scalarStyle(value string, style yaml.Style) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value, Style: style}
}

func appendKV(m *yaml.Node, k string, v *yaml.Node) {
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, v)
}

// ---------- graph emission ----------

type rendered struct {
	intermediateRootMG string
	mgByName           map[string]string
	resources          []*yaml.Node
	usedDefs           map[string]bool
}

func (r *rendered) addResource(id string, template *yaml.Node) {
	r.resources = append(r.resources, mapping("id", id, "template", template))
}

func buildGraph(in *Input, cat *Catalogue, mgs []effectiveMG) (*yaml.Node, error) {
	r := &rendered{mgByName: map[string]string{}, usedDefs: map[string]bool{}}

	for _, mg := range mgs {
		r.mgByName[mg.Name] = mgID(mg.Name)
		if mg.Parent == "root" {
			if r.intermediateRootMG != "" {
				return nil, fmt.Errorf("more than one MG has parent: root (%s and %s)", r.intermediateRootMG, mgID(mg.Name))
			}
			r.intermediateRootMG = mgID(mg.Name)
		}
	}
	if r.intermediateRootMG == "" {
		return nil, fmt.Errorf("no management group has parent: root")
	}

	// 1. Emit ManagementGroup CRs.
	for _, mg := range mgs {
		display := mg.DisplayName
		if display == "" {
			display = strings.ToUpper(mg.Name[:1]) + mg.Name[1:]
		}
		var parentRef *yaml.Node
		if mg.Parent == "root" {
			parentRef = n("${schema.spec.parentResourceId}")
		} else {
			parentRef = n(fmt.Sprintf("${%s.status.atProvider.id}", r.mgByName[mg.Parent]))
		}
		r.addResource(mgID(mg.Name), mapping(
			"apiVersion", "management.azure.upbound.io/v1beta1",
			"kind", "ManagementGroup",
			"metadata", mapping("name", fmt.Sprintf("${schema.spec.prefix}-%s", kebab(mg.Name))),
			"spec", mapping("forProvider", mapping(
				"displayName", display,
				"parentManagementGroupId", parentRef,
			)),
		))
	}

	// 2. Compose each MG's effective assignment list and emit assignments.
	type pair struct {
		mg     effectiveMG
		assign string
	}
	var pairs []pair
	for _, mg := range mgs {
		assignSet := map[string]bool{}
		var assignOrder []string
		for _, arch := range mg.Archetypes {
			as, err := composeArchetypeAssignments(in, cat, arch)
			if err != nil {
				return nil, fmt.Errorf("MG %s: %w", mg.Name, err)
			}
			for _, a := range as {
				if !assignSet[a] {
					assignSet[a] = true
					assignOrder = append(assignOrder, a)
				}
			}
		}
		// Apply policyAssignmentsToDisable for this MG.
		for _, dis := range in.PolicyAssignmentsToDisable[mg.Name] {
			if _, ok := assignSet[dis]; !ok {
				log.Printf("warn: policyAssignmentsToDisable[%s] lists %q which is not inherited from any archetype", mg.Name, dis)
				continue
			}
			delete(assignSet, dis)
		}
		for _, a := range assignOrder {
			if !assignSet[a] {
				continue
			}
			pairs = append(pairs, pair{mg, a})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].mg.Name != pairs[j].mg.Name {
			return pairs[i].mg.Name < pairs[j].mg.Name
		}
		return pairs[i].assign < pairs[j].assign
	})

	for _, p := range pairs {
		composed, err := composeAssignment(in, cat, p.mg.Name, p.assign)
		if err != nil {
			return nil, err
		}
		props, _ := composed["properties"].(map[string]interface{})

		pdID, _ := props["policyDefinitionId"].(string)
		var policyDefRef interface{}
		if m := customDefRefRE.FindStringSubmatch(pdID); len(m) == 2 {
			defName := m[1]
			if _, isCustom := cat.Definitions[defName]; isCustom {
				policyDefRef = fmt.Sprintf("${%s.status.atProvider.id}", defID(defName))
				r.usedDefs[defName] = true
			}
		}
		if policyDefRef == nil {
			if !builtInDefRE.MatchString(pdID) {
				if m := customSetRefRE.FindStringSubmatch(pdID); len(m) == 2 {
					log.Printf("warn: assignment %s on MG %s targets custom policy set %q which is not vendored (policy set definitions are out of scope); leaving as-is", p.assign, p.mg.Name, m[1])
				} else {
					log.Printf("warn: assignment %s on MG %s has policyDefinitionId %q that is neither a built-in nor a vendored custom def; leaving as-is", p.assign, p.mg.Name, pdID)
				}
			}
			policyDefRef = pdID
		}

		loc, _ := composed["location"].(string)
		if loc == "" || loc == "${default_location}" {
			loc = "${schema.spec.location}"
		} else {
			loc = defaultLocRE.ReplaceAllString(loc, "${schema.spec.location}")
		}

		forProvider := mapping(
			"displayName", asString(props["displayName"]),
			"managementGroupId", fmt.Sprintf("${%s.status.atProvider.id}", r.mgByName[p.mg.Name]),
			"policyDefinitionId", policyDefRef,
		)
		if desc := asString(props["description"]); desc != "" {
			appendKV(forProvider, "description", n(desc))
		}
		appendKV(forProvider, "location", n(loc))
		if id, ok := composed["identity"].(map[string]interface{}); ok {
			if t, ok := id["type"].(string); ok && t != "" {
				appendKV(forProvider, "identity", seq(mapping("type", t)))
			}
		}
		if em, _ := props["enforcementMode"].(string); em != "" && em != "Default" {
			appendKV(forProvider, "enforcementMode", n(em))
		}
		if params, _ := props["parameters"].(map[string]interface{}); len(params) > 0 {
			pb, _ := json.MarshalIndent(params, "", "  ")
			appendKV(forProvider, "parameters", scalarStyle(string(pb), yaml.LiteralStyle))
		}

		r.addResource(paID(p.mg.Name, p.assign), mapping(
			"apiVersion", "authorization.azure.upbound.io/v1beta1",
			"kind", "ManagementGroupPolicyAssignment",
			"metadata", mapping("name", fmt.Sprintf("${schema.spec.prefix}-%s-%s", kebab(p.mg.Name), kebab(p.assign))),
			"spec", mapping("forProvider", forProvider),
		))
	}

	// 3. Emit only the custom PolicyDefinitions actually referenced.
	defNames := make([]string, 0, len(r.usedDefs))
	for n := range r.usedDefs {
		defNames = append(defNames, n)
	}
	sort.Strings(defNames)

	// We need to splice PolicyDefinition resources *before* the assignments
	// in the rendered file so a reader can read top-to-bottom in DAG order.
	// Rebuild r.resources accordingly.
	mgRes := r.resources[:len(mgs)]
	assignRes := r.resources[len(mgs):]
	r.resources = append([]*yaml.Node{}, mgRes...)
	for _, name := range defNames {
		def := cat.Definitions[name]
		props, _ := def["properties"].(map[string]interface{})
		if props == nil {
			return nil, fmt.Errorf("definition %s missing properties", name)
		}
		policyRule := props["policyRule"]
		if policyRule == nil {
			return nil, fmt.Errorf("definition %s missing policyRule", name)
		}
		policyType := asString(props["policyType"])
		if policyType == "" {
			policyType = "Custom"
		}
		mode := asString(props["mode"])
		if mode == "" {
			mode = "All"
		}
		fp := mapping(
			"displayName", asString(props["displayName"]),
			"description", asString(props["description"]),
			"mode", mode,
			"policyType", policyType,
			"managementGroupId", fmt.Sprintf("${%s.status.atProvider.id}", r.intermediateRootMG),
			"policyRule", scalarStyle(mustJSON(policyRule), yaml.LiteralStyle),
		)
		if params, ok := props["parameters"].(map[string]interface{}); ok && len(params) > 0 {
			appendKV(fp, "parameters", scalarStyle(mustJSON(params), yaml.LiteralStyle))
		}
		if meta, ok := props["metadata"].(map[string]interface{}); ok && len(meta) > 0 {
			appendKV(fp, "metadata", scalarStyle(mustJSON(meta), yaml.LiteralStyle))
		}
		r.resources = append(r.resources, mapping("id", defID(name), "template", mapping(
			"apiVersion", "authorization.azure.upbound.io/v1beta1",
			"kind", "PolicyDefinition",
			"metadata", mapping("name", fmt.Sprintf("${schema.spec.prefix}-%s", kebab(name))),
			"spec", mapping("forProvider", fp),
		)))
	}
	r.resources = append(r.resources, assignRes...)

	// 4. Build the schema spec.
	specPairs := []interface{}{
		"parentResourceId", "string | required=true",
		"prefix", fmt.Sprintf("string | required=true default=%q", in.Prefix),
		"location", fmt.Sprintf("string | required=true default=%q", in.Location),
	}

	resourcesSeq := &yaml.Node{Kind: yaml.SequenceNode}
	resourcesSeq.Content = r.resources

	root := mapping(
		"apiVersion", "kro.run/v1alpha1",
		"kind", "ResourceGraphDefinition",
		"metadata", mapping("name", "azure-landing-zone"),
		"spec", mapping(
			"schema", mapping(
				"apiVersion", "iac.experiment/v1alpha1",
				"kind", "AzureLandingZone",
				"spec", mapping(specPairs...),
			),
			"resources", resourcesSeq,
		),
	)
	return root, nil
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

// ---------- catalogue loading ----------

func loadCatalogue(in *Input) (*Catalogue, error) {
	if len(in.Libraries) == 0 {
		return nil, fmt.Errorf("at least one entry in `libraries` is required")
	}
	listings := make([]*libraryFiles, 0, len(in.Libraries))
	for _, lib := range in.Libraries {
		log.Printf("listing library %s@%s path=%s", lib.Repo, lib.Ref, lib.Path)
		l, err := fetchLibraryListing(lib)
		if err != nil {
			return nil, err
		}
		log.Printf("  -> %d architectures, %d archetypes, %d assignments, %d definitions, defaults=%t",
			len(l.architectures), len(l.archetypes), len(l.assignments), len(l.definitions), l.defaultsURL != "")
		listings = append(listings, l)
	}
	merged := mergeListings(listings)

	cat := newCatalogue()

	// Fetch architectures (we only need the base architecture in practice; fetch all anyway for completeness).
	for name, url := range merged.architectures {
		body, err := httpGetBytes(url)
		if err != nil {
			return nil, fmt.Errorf("fetch architecture %s: %w", name, err)
		}
		var doc ArchitectureDoc
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parse architecture %s: %w", name, err)
		}
		cat.Architectures[name] = &doc
	}
	for name, url := range merged.archetypes {
		body, err := httpGetBytes(url)
		if err != nil {
			return nil, fmt.Errorf("fetch archetype %s: %w", name, err)
		}
		var doc ArchetypeDoc
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parse archetype %s: %w", name, err)
		}
		cat.Archetypes[name] = &doc
	}
	// Defer fetching assignments and definitions until we know which ones are needed.
	// We fetch what's referenced by the effective MG/archetype set in two phases below.

	defaults, err := loadDefaults(listings)
	if err != nil {
		return nil, err
	}
	cat.Defaults = defaults

	// Expose URL maps to callers via private fields on the catalogue.
	cat.assignURLs = merged.assignments
	cat.defURLs = merged.definitions
	return cat, nil
}

// Append to Catalogue: URL maps used for lazy fetch.
// (Declared as fields with no JSON/YAML tags; ignored by encoders.)
// We declare them here as a workaround for not having a constructor on the struct above.

// ---------- entry ----------

func (c *Catalogue) fetchAssignment(name string) error {
	if _, ok := c.Assignments[name]; ok {
		return nil
	}
	url, ok := c.assignURLs[name]
	if !ok {
		return fmt.Errorf("assignment %s not present in merged library set", name)
	}
	body, err := httpGetBytes(url)
	if err != nil {
		return fmt.Errorf("fetch assignment %s: %w", name, err)
	}
	var raw rawJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("parse assignment %s: %w", name, err)
	}
	c.Assignments[name] = raw
	return nil
}

func (c *Catalogue) fetchDefinition(name string) error {
	if _, ok := c.Definitions[name]; ok {
		return nil
	}
	url, ok := c.defURLs[name]
	if !ok {
		return fmt.Errorf("definition %s not present in merged library set", name)
	}
	body, err := httpGetBytes(url)
	if err != nil {
		return fmt.Errorf("fetch definition %s: %w", name, err)
	}
	var raw rawJSON
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("parse definition %s: %w", name, err)
	}
	c.Definitions[name] = raw
	return nil
}

func main() {
	var (
		inPath  = flag.String("in", "", "path to landingzone.yaml input")
		outPath = flag.String("out", "", "path to write the rendered ResourceGraphDefinition (default: stdout)")
	)
	flag.Parse()
	if *inPath == "" {
		log.Fatal("-in is required")
	}

	inBytes, err := os.ReadFile(*inPath)
	if err != nil {
		log.Fatalf("read input: %v", err)
	}
	var input Input
	if err := yaml.Unmarshal(inBytes, &input); err != nil {
		log.Fatalf("parse input: %v", err)
	}

	cat, err := loadCatalogue(&input)
	if err != nil {
		log.Fatalf("load catalogue: %v", err)
	}

	mgs, err := composeMGs(&input, cat)
	if err != nil {
		log.Fatalf("compose mgs: %v", err)
	}
	log.Printf("effective MG count: %d", len(mgs))

	// Pre-fetch all assignments referenced by any (mg, archetype) pair so the
	// graph build below works in-memory.
	needAssign := map[string]bool{}
	for _, mg := range mgs {
		for _, arch := range mg.Archetypes {
			as, err := composeArchetypeAssignments(&input, cat, arch)
			if err != nil {
				log.Fatalf("compose: %v", err)
			}
			for _, a := range as {
				needAssign[a] = true
			}
		}
	}
	log.Printf("fetching %d referenced assignments...", len(needAssign))
	for a := range needAssign {
		if err := cat.fetchAssignment(a); err != nil {
			log.Fatalf("%v", err)
		}
	}

	// Determine which custom definitions are referenced by those assignments and fetch them.
	needDef := map[string]bool{}
	for _, a := range cat.Assignments {
		props, _ := a["properties"].(map[string]interface{})
		if props == nil {
			continue
		}
		pid, _ := props["policyDefinitionId"].(string)
		if m := customDefRefRE.FindStringSubmatch(pid); len(m) == 2 {
			if _, present := cat.defURLs[m[1]]; present {
				needDef[m[1]] = true
			}
		}
	}
	log.Printf("fetching %d referenced custom policy definitions...", len(needDef))
	for d := range needDef {
		if err := cat.fetchDefinition(d); err != nil {
			log.Fatalf("%v", err)
		}
	}

	graph, err := buildGraph(&input, cat, mgs)
	if err != nil {
		log.Fatalf("build graph: %v", err)
	}

	out := os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			log.Fatalf("create output: %v", err)
		}
		defer f.Close()
		out = f
	}

	// Header.
	header := "# Generated by tools/render from landingzone.yaml.\n# Libraries:\n"
	for _, lib := range input.Libraries {
		header += fmt.Sprintf("#   - %s @ %s (%s)\n", lib.Repo, lib.Ref, lib.Path)
	}
	header += "# DO NOT EDIT BY HAND — re-run `go run ./tools/render -in <input> -out <output>`.\n"
	if _, err := out.WriteString(header); err != nil {
		log.Fatalf("write header: %v", err)
	}
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	if err := enc.Encode(graph); err != nil {
		log.Fatalf("encode: %v", err)
	}
	enc.Close()
}

// ---------- Catalogue private fields ----------

// We extend Catalogue with private URL maps so the entry-point can lazy-fetch.
// Declared here (file-scope) using Go's ability to attach extra fields via a
// type alias trick is not possible; instead we add the fields directly to the
// struct above and reference them here.

// (Catalogue is augmented inline above with assignURLs/defURLs fields below.)
