package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestName(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(entities))
	assert.Equal(t, "devel-nick-lb-blorg-be", entities[0].Name())
	assert.Equal(t, "devel-nick-blorg-be", entities[1].Name())
}

func TestNamespace(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SyncletYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(entities))
	assert.Equal(t, "kube-system", string(entities[0].Namespace()))
}

func TestImmutableFilter(t *testing.T) {
	yaml := fmt.Sprintf("%s\n---\n%s\n---\n%s", testyaml.JobYAML, testyaml.SanchoYAML, testyaml.PodYAML)
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	immEntities := ImmutableEntities(entities)
	if len(immEntities) != 2 {
		t.Fatalf("Expected 2 entities, actual: %d", len(immEntities))
	}

	if immEntities[0].Kind.Kind != "Job" {
		t.Errorf("Expected Job entity, actual: %+v", immEntities)
	}
	if immEntities[1].Kind.Kind != "Pod" {
		t.Errorf("Expected Pod entity, actual: %+v", immEntities)
	}
}

func TestMutableFilter(t *testing.T) {
	yaml := fmt.Sprintf("%s\n---\n%s", testyaml.JobYAML, testyaml.SanchoYAML)
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	results := MutableEntities(entities)
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, actual: %d", len(results))
	}

	if results[0].Kind.Kind != "Deployment" {
		t.Errorf("Expected Deployment entity, actual: %+v", results)
	}
}

func TestLoadBalancerSpecs(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	lbs := ToLoadBalancerSpecs(entities)
	if len(lbs) != 1 {
		t.Fatalf("Expected 1 loadbalancer, actual %d: %v", len(lbs), lbs)
	}

	if lbs[0].Name != "devel-nick-lb-blorg-be" ||
		lbs[0].Ports[0] != 8080 {
		t.Fatalf("Unexpected loadbalancer: %+v", lbs[0])
	}
}

func TestFilter(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.BlorgBackendYAML, testyaml.BlorgJobYAML)
	if err != nil {
		t.Fatal(err)
	}

	test := func(e K8sEntity) (bool, error) {
		if e.Kind.Kind == "Deployment" || e.Kind.Kind == "Job" {
			return true, nil
		}
		return false, nil
	}

	popped, rest, err := Filter(entities, test)
	if err != nil {
		t.Fatal(err)
	}
	expectedPopped := []K8sEntity{entities[1], entities[2]} // deployment, job
	expectedRest := []K8sEntity{entities[0]}                // service
	assert.Equal(t, popped, expectedPopped)
	assert.Equal(t, rest, expectedRest)

	returnFalse := func(e K8sEntity) (bool, error) { return false, nil }
	popped, rest, err = Filter(entities, returnFalse)
	if err != nil {
		t.Fatal(err)
	}
	assert.Empty(t, popped)
	assert.Equal(t, rest, entities)

	returnErr := func(e K8sEntity) (bool, error) {
		return false, fmt.Errorf("omgwtfbbq")
	}
	popped, rest, err = Filter(entities, returnErr)
	if assert.Error(t, err, "expected Filter to propagate err from test func") {
		assert.Equal(t, err.Error(), "omgwtfbbq")
	}
}

func TestHasName(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entites, got %d: %v", len(entities), entities)
	}

	doggos := entities[0]
	assert.True(t, doggos.HasName(testyaml.DoggosName))

	snack := entities[1]
	assert.False(t, snack.HasName(testyaml.DoggosName))
}

func TestHasNamespace(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entites, got %d: %v", len(entities), entities)
	}

	doggos := entities[0]
	assert.True(t, doggos.HasNamespace(testyaml.DoggosNamespace))

	snack := entities[1]
	assert.False(t, snack.HasNamespace(testyaml.DoggosNamespace))
}

func TestHasKind(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entites, got %d: %v", len(entities), entities)
	}

	depl := entities[0]
	assert.True(t, depl.HasKind("deployment"))
	assert.False(t, depl.HasKind("service"))

	svc := entities[1]
	assert.False(t, svc.HasKind("deployment"))
	assert.True(t, svc.HasKind("service"))
}

func TestEntitiesWithDependenciesAndRest(t *testing.T) {
	eDeploy := mustParseYAML(t, testyaml.DoggosDeploymentYaml)[0]
	eService := mustParseYAML(t, testyaml.DoggosServiceYaml)[0]
	eCRD := mustParseYAML(t, testyaml.CRDYAML)[0]
	eNamespace := mustParseYAML(t, testyaml.MyNamespaceYAML)[0]

	for _, test := range []struct {
		name                   string
		input                  []K8sEntity
		expectedWithDependents []K8sEntity
		expectedRest           []K8sEntity
	}{
		{"one namespace",
			[]K8sEntity{eDeploy, eNamespace, eService},
			[]K8sEntity{eNamespace},
			[]K8sEntity{eDeploy, eService},
		},
		{"one crd",
			[]K8sEntity{eDeploy, eCRD, eService},
			[]K8sEntity{eCRD},
			[]K8sEntity{eDeploy, eService},
		},
		{"namespace and crd",
			[]K8sEntity{eDeploy, eCRD, eService, eNamespace},
			[]K8sEntity{eNamespace, eCRD},
			[]K8sEntity{eDeploy, eService},
		},
		{"namespace and crd preserves order of rest",
			[]K8sEntity{eService, eCRD, eDeploy, eNamespace},
			[]K8sEntity{eNamespace, eCRD},
			[]K8sEntity{eService, eDeploy},
		},
		{"none with dependents",
			[]K8sEntity{eDeploy, eService},
			nil,
			[]K8sEntity{eDeploy, eService},
		},
		{"only with dependents",
			[]K8sEntity{eNamespace, eCRD},
			[]K8sEntity{eNamespace, eCRD},
			nil,
		},
		{"namespace first",
			[]K8sEntity{eCRD, eNamespace},
			[]K8sEntity{eNamespace, eCRD},
			nil,
		},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			withDependents, rest := EntitiesWithDependentsAndRest(test.input)
			assert.Equal(t, test.expectedWithDependents, withDependents)
			assert.Equal(t, test.expectedRest, rest)
		})
	}
}

func parseYAMLFromStrings(yaml ...string) ([]K8sEntity, error) {
	var res []K8sEntity
	for _, s := range yaml {
		entities, err := ParseYAMLFromString(s)
		if err != nil {
			return nil, err
		}
		res = append(res, entities...)
	}
	return res, nil
}

func mustParseYAML(t *testing.T, yaml string) []K8sEntity {
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatalf("ERROR %v parsing k8s YAML:\n%s", err, yaml)
	}
	return entities
}
