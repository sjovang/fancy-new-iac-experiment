// Renderer: reads a high-level landing-zone authoring file plus the upstream
// Azure-Landing-Zones-Library at a pinned commit SHA, and emits a complete
// kro ResourceGraphDefinition.
//
// The graph chains its DAG entirely through CEL refs:
//   - child ManagementGroup -> parent ManagementGroup
//   - PolicyAssignment      -> its ManagementGroup
//   - PolicyAssignment      -> custom PolicyDefinition (when the upstream
//     assignment targets a definition that lives in the library, not a
//     built-in GUID)
//   - PolicyDefinition      -> intermediate-root ManagementGroup (scope)
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
	Prefix           string                 `yaml:"prefix"`
	RootParentID     string                 `yaml:"rootParentId"`
	Location         string                 `yaml:"location"`
	LibraryRef       string                 `yaml:"libraryRef"`
	Inputs           map[string]string      `yaml:"inputs"`
	ManagementGroups []ManagementGroupInput `yaml:"managementGroups"`
}

type ManagementGroupInput struct {
	Name              string             `yaml:"name"`
	DisplayName       string             `yaml:"displayName"`
	Parent            string             `yaml:"parent"`
	PolicyAssignments []AssignmentInput  `yaml:"policyAssignments"`
}

type AssignmentInput struct {
	Name       string                 `yaml:"name"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// ---------- library payloads ----------

type libDef struct {
	Name       string                 `json:"name"`
	Properties libDefProperties       `json:"properties"`
	Type       string                 `json:"type"`
	raw        map[string]interface{} `json:"-"`
}

type libDefProperties struct {
	DisplayName string                 `json:"displayName"`
	Description string                 `json:"description"`
	Mode        string                 `json:"mode"`
	PolicyType  string                 `json:"policyType"`
	PolicyRule  interface{}            `json:"policyRule"`
	Parameters  map[string]interface{} `json:"parameters"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type libAssign struct {
	Name       string                 `json:"name"`
	Location   string                 `json:"location"`
	Identity   map[string]interface{} `json:"identity"`
	Properties libAssignProperties    `json:"properties"`
}

type libAssignProperties struct {
	DisplayName        string                            `json:"displayName"`
	Description        string                            `json:"description"`
	PolicyDefinitionID string                            `json:"policyDefinitionId"`
	Parameters         map[string]libAssignParameter     `json:"parameters"`
	EnforcementMode    string                            `json:"enforcementMode"`
	NonComplianceMsgs  []map[string]interface{}          `json:"nonComplianceMessages"`
}

type libAssignParameter struct {
	Value interface{} `json:"value"`
}

// ---------- ordered YAML emission ----------

// We hand-build YAML as ordered MapSlices so the output is human-reviewable.

// orderedMap is yaml.MapSlice-equivalent for yaml.v3.
type orderedMap = yaml.Node

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

// scalar with a forced style (e.g. literal block for embedded JSON).
func scalarStyle(value string, style yaml.Style) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value, Style: style}
}

// ---------- helpers ----------

var (
	customDefRefRE = regexp.MustCompile(`/providers/Microsoft\.Management/managementGroups/[^/]+/providers/Microsoft\.Authorization/policyDefinitions/([^/]+)$`)
	builtInDefRE   = regexp.MustCompile(`^/providers/Microsoft\.Authorization/policy(Set)?Definitions/[^/]+$`)
	inputRefRE     = regexp.MustCompile(`\$\{inputs\.([A-Za-z0-9_]+)\}`)
	defaultLocRE   = regexp.MustCompile(`\$\{default_location\}`)
)

func mustJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
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

// ---------- library fetch ----------

type libraryTree struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
	Truncated bool `json:"truncated"`
}

func fetchLibrary(ref string) (defs map[string]*libDef, assigns map[string]*libAssign, err error) {
	treeURL := fmt.Sprintf("https://api.github.com/repos/Azure/Azure-Landing-Zones-Library/git/trees/%s?recursive=1", ref)
	var tree libraryTree
	if err = httpGetJSON(treeURL, &tree); err != nil {
		return nil, nil, fmt.Errorf("fetch library tree: %w", err)
	}
	if tree.Truncated {
		return nil, nil, fmt.Errorf("library tree truncated at ref %s; need a non-truncated listing", ref)
	}

	defs = map[string]*libDef{}
	assigns = map[string]*libAssign{}

	rawBase := fmt.Sprintf("https://raw.githubusercontent.com/Azure/Azure-Landing-Zones-Library/%s/", ref)

	for _, e := range tree.Tree {
		if e.Type != "blob" {
			continue
		}
		switch {
		case strings.HasPrefix(e.Path, "platform/alz/policy_definitions/") && strings.HasSuffix(e.Path, ".alz_policy_definition.json"):
			body, err := httpGetBytes(rawBase + e.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("fetch %s: %w", e.Path, err)
			}
			var d libDef
			if err := json.Unmarshal(body, &d); err != nil {
				return nil, nil, fmt.Errorf("parse %s: %w", e.Path, err)
			}
			// We also keep the raw map so we can re-serialise faithfully.
			var rm map[string]interface{}
			_ = json.Unmarshal(body, &rm)
			d.raw = rm
			defs[d.Name] = &d
		case strings.HasPrefix(e.Path, "platform/alz/policy_assignments/") && strings.HasSuffix(e.Path, ".alz_policy_assignment.json"):
			body, err := httpGetBytes(rawBase + e.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("fetch %s: %w", e.Path, err)
			}
			var a libAssign
			if err := json.Unmarshal(body, &a); err != nil {
				return nil, nil, fmt.Errorf("parse %s: %w", e.Path, err)
			}
			assigns[a.Name] = &a
		}
	}
	return defs, assigns, nil
}

// ---------- id helpers ----------

func mgID(name string) string             { return "mg" + camel(name) }
func defID(name string) string            { return "pd" + camel(name) }
func paID(mg, assign string) string       { return "pa" + camel(mg) + camel(assign) }

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

// kebab combines a prefix and a name into a kubernetes-safe lowercase name.
func kebab(parts ...string) string {
	joined := strings.Join(parts, "-")
	joined = strings.ToLower(joined)
	joined = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(joined, "-")
	joined = regexp.MustCompile(`-+`).ReplaceAllString(joined, "-")
	return strings.Trim(joined, "-")
}

// ---------- substitution ----------

// substituteInputs replaces ${inputs.foo} with ${schema.spec.inputs.foo}.
func substituteInputs(s string) string {
	return inputRefRE.ReplaceAllString(s, "${schema.spec.inputs.$1}")
}

// substituteLocation replaces ${default_location} with ${schema.spec.location}.
func substituteLocation(s string) string {
	return defaultLocRE.ReplaceAllString(s, "${schema.spec.location}")
}

// mergeParams merges library defaults with user overrides into the
// {"name": {"value": <v>}} shape Azure expects, serialised as JSON.
func mergeParams(lib map[string]libAssignParameter, override map[string]interface{}) string {
	out := map[string]map[string]interface{}{}
	for k, v := range lib {
		out[k] = map[string]interface{}{"value": v.Value}
	}
	for k, v := range override {
		out[k] = map[string]interface{}{"value": v}
	}
	if len(out) == 0 {
		return ""
	}
	// Marshal then run input substitution over the JSON text so values like
	// "${inputs.ddosPlanId}" become "${schema.spec.inputs.ddosPlanId}".
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	return substituteInputs(string(b))
}

// ---------- graph emission ----------

type rendered struct {
	intermediateRootMG string // resource id of the highest MG in the input; defs scope here
	mgByName           map[string]string
	resources          []*yaml.Node
}

func (r *rendered) addResource(id string, template *yaml.Node) {
	r.resources = append(r.resources, mapping("id", id, "template", template))
}

func buildGraph(in *Input, defs map[string]*libDef, assigns map[string]*libAssign) (*yaml.Node, error) {
	r := &rendered{mgByName: map[string]string{}}

	// First pass: assign resource ids to all MGs and find the intermediate root
	// (the MG whose parent is "root").
	for _, mg := range in.ManagementGroups {
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

	// Emit ManagementGroup resources in input order (parents must be listed
	// before children in the input; kro builds the DAG from CEL refs anyway).
	for _, mg := range in.ManagementGroups {
		display := mg.DisplayName
		if display == "" {
			// Title-case the MG name as a sensible default ("platform" -> "Platform").
			display = strings.ToUpper(mg.Name[:1]) + mg.Name[1:]
		}
		var parentRef *yaml.Node
		if mg.Parent == "root" {
			parentRef = n("${schema.spec.rootParentId}")
		} else {
			parentID, ok := r.mgByName[mg.Parent]
			if !ok {
				return nil, fmt.Errorf("management group %q references unknown parent %q", mg.Name, mg.Parent)
			}
			parentRef = n(fmt.Sprintf("${%s.status.atProvider.id}", parentID))
		}
		r.addResource(mgID(mg.Name), mapping(
			"apiVersion", "management.azure.upbound.io/v1beta1",
			"kind", "ManagementGroup",
			"metadata", mapping("name", fmt.Sprintf("${schema.spec.prefix}-%s", mg.Name)),
			"spec", mapping("forProvider", mapping(
				"displayName", display,
				"parentManagementGroupId", parentRef,
			)),
		))
	}

	// Emit one PolicyDefinition resource per *custom* definition in the
	// library. Scope each one at the intermediate root MG so child MGs can
	// reference it.
	defNames := make([]string, 0, len(defs))
	for k := range defs {
		defNames = append(defNames, k)
	}
	sort.Strings(defNames)
	for _, name := range defNames {
		d := defs[name]
		if !strings.EqualFold(d.Properties.PolicyType, "Custom") {
			continue
		}
		policyRuleJSON := mustJSON(d.Properties.PolicyRule)
		paramsJSON := mustJSON(d.Properties.Parameters)
		metadataJSON := ""
		if len(d.Properties.Metadata) > 0 {
			metadataJSON = mustJSON(d.Properties.Metadata)
		}

		forProvider := mapping(
			"displayName", d.Properties.DisplayName,
			"description", d.Properties.Description,
			"mode", firstNonEmpty(d.Properties.Mode, "All"),
			"policyType", "Custom",
			"managementGroupId", fmt.Sprintf("${%s.status.atProvider.id}", r.intermediateRootMG),
			"policyRule", scalarStyle(policyRuleJSON, yaml.LiteralStyle),
			"parameters", scalarStyle(paramsJSON, yaml.LiteralStyle),
		)
		if metadataJSON != "" {
			forProvider.Content = append(forProvider.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "metadata"},
				scalarStyle(metadataJSON, yaml.LiteralStyle),
			)
		}

		r.addResource(defID(name), mapping(
			"apiVersion", "authorization.azure.upbound.io/v1beta1",
			"kind", "PolicyDefinition",
			"metadata", mapping("name", fmt.Sprintf("${schema.spec.prefix}-%s", kebab(name))),
			"spec", mapping("forProvider", forProvider),
		))
	}

	// Emit ManagementGroupPolicyAssignment per (mg, assignment) pair, in a
	// deterministic order.
	type pair struct {
		mg     ManagementGroupInput
		assign AssignmentInput
	}
	var pairs []pair
	for _, mg := range in.ManagementGroups {
		for _, a := range mg.PolicyAssignments {
			pairs = append(pairs, pair{mg, a})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].mg.Name != pairs[j].mg.Name {
			return pairs[i].mg.Name < pairs[j].mg.Name
		}
		return pairs[i].assign.Name < pairs[j].assign.Name
	})

	for _, p := range pairs {
		lib, ok := assigns[p.assign.Name]
		if !ok {
			return nil, fmt.Errorf("assignment %q on MG %q not found in upstream library", p.assign.Name, p.mg.Name)
		}

		// Resolve policy definition id: custom (in our defs map) -> CEL ref;
		// otherwise pass through as built-in ARM id.
		pdID := lib.Properties.PolicyDefinitionID
		var policyDefRef interface{}
		if m := customDefRefRE.FindStringSubmatch(pdID); len(m) == 2 {
			defName := m[1]
			if _, isCustom := defs[defName]; isCustom {
				policyDefRef = fmt.Sprintf("${%s.status.atProvider.id}", defID(defName))
			}
		}
		if policyDefRef == nil {
			if !builtInDefRE.MatchString(pdID) {
				log.Printf("warn: assignment %s on MG %s has policyDefinitionId %q that is neither a built-in nor a vendored custom def; leaving as-is", p.assign.Name, p.mg.Name, pdID)
			}
			policyDefRef = pdID
		}

		forProvider := mapping(
			"displayName", lib.Properties.DisplayName,
			"managementGroupId", fmt.Sprintf("${%s.status.atProvider.id}", r.mgByName[p.mg.Name]),
			"policyDefinitionId", policyDefRef,
		)
		if lib.Properties.Description != "" {
			forProvider.Content = append(forProvider.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "description"},
				n(lib.Properties.Description),
			)
		}
		loc := lib.Location
		if loc == "" || loc == "${default_location}" {
			loc = "${schema.spec.location}"
		} else {
			loc = substituteLocation(loc)
		}
		forProvider.Content = append(forProvider.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "location"},
			n(loc),
		)
		if lib.Identity != nil {
			if t, ok := lib.Identity["type"].(string); ok && t != "" {
				forProvider.Content = append(forProvider.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: "identity"},
					seq(mapping("type", t)),
				)
			}
		}
		if params := mergeParams(lib.Properties.Parameters, p.assign.Parameters); params != "" {
			forProvider.Content = append(forProvider.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "parameters"},
				scalarStyle(params, yaml.LiteralStyle),
			)
		}

		r.addResource(paID(p.mg.Name, p.assign.Name), mapping(
			"apiVersion", "authorization.azure.upbound.io/v1beta1",
			"kind", "ManagementGroupPolicyAssignment",
			"metadata", mapping("name", fmt.Sprintf("${schema.spec.prefix}-%s-%s", kebab(p.mg.Name), kebab(p.assign.Name))),
			"spec", mapping("forProvider", forProvider),
		))
	}

	// Build the schema spec.
	specPairs := []interface{}{
		"rootParentId", "string | required=true",
		"prefix", fmt.Sprintf("string | required=true default=%q", in.Prefix),
		"location", fmt.Sprintf("string | required=true default=%q", in.Location),
	}
	if len(in.Inputs) > 0 {
		inputKeys := make([]string, 0, len(in.Inputs))
		for k := range in.Inputs {
			inputKeys = append(inputKeys, k)
		}
		sort.Strings(inputKeys)
		inputsMap := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range inputKeys {
			inputsMap.Content = append(inputsMap.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: k},
				n(fmt.Sprintf("string | default=%q", in.Inputs[k])),
			)
		}
		specPairs = append(specPairs, "inputs", inputsMap)
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------- main ----------

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
	if input.LibraryRef == "" {
		log.Fatal("libraryRef is required in the input file")
	}

	log.Printf("fetching Azure-Landing-Zones-Library @ %s", input.LibraryRef)
	defs, assigns, err := fetchLibrary(input.LibraryRef)
	if err != nil {
		log.Fatalf("fetch library: %v", err)
	}
	log.Printf("library: %d policy_definitions, %d policy_assignments", len(defs), len(assigns))

	graph, err := buildGraph(&input, defs, assigns)
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

	header := fmt.Sprintf("# Generated by tools/render from landingzone.yaml.\n# Library ref: Azure/Azure-Landing-Zones-Library@%s\n# DO NOT EDIT BY HAND — re-run `go run ./tools/render -in <input> -out <output>`.\n", input.LibraryRef)
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
