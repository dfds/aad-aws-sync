package k8s

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestLoadRoleMapResponse_GetMappingByArn(t *testing.T) {
	l := &LoadRoleMapResponse{
		Mappings: []*RoleMapping{
			{
				RoleARN:     "arn:aws:iam::12345678910:role/dummy",
				ManagedBy:   "aad-aws-sync",
				LastUpdated: "-1",
				CreatedAt:   "-1",
				Username:    "dummy-sso",
				Groups: []string{
					"system:view",
				},
			},
		},
		ConfigMap: &v1.ConfigMap{},
	}

	mapping := l.GetMappingByArn("arn:aws:iam::12345678910:role/dummy")

	if mapping == nil {
		t.Error("GetMappingByArn returned nil when pointer was expected")
		t.FailNow()
	}

	mappingsExpectedToFail := []string{"dummy", "arn:aws:iam::12345678910:role/dummyx", "arn:aws:iam::12345628910:role/dummy"}

	for _, m := range mappingsExpectedToFail {
		mapping = l.GetMappingByArn(m)
		if mapping != nil {
			t.Error("Expected nil response from GetMappingByArn, but pointer was returned")
			t.FailNow()
		}
	}

}

func TestRoleMapping_ContainsGroup(t *testing.T) {
	rm := &RoleMapping{
		RoleARN:     "arn:aws:iam::12345678910:role/dummy",
		ManagedBy:   "aad-aws-sync",
		LastUpdated: "-1",
		CreatedAt:   "-1",
		Username:    "dummy-sso",
		Groups: []string{
			"system:view",
		},
	}

	if !rm.ContainsGroup("system:view") {
		t.Error("ContainsGroup returned false, when true was expected")
	}

	groupsToFail := []string{"dummy", "system:admin"}

	for _, m := range groupsToFail {
		if rm.ContainsGroup(m) {
			t.Error("ContainsGroup returned true, when false was expected")
			t.FailNow()
		}
	}

}

func TestRoleMapping_ManagedByThis(t *testing.T) {
	rm := &RoleMapping{
		RoleARN:     "arn:aws:iam::12345678910:role/dummy",
		ManagedBy:   "aad-aws-sync",
		LastUpdated: "-1",
		CreatedAt:   "-1",
		Username:    "dummy-sso",
		Groups: []string{
			"system:view",
		},
	}

	assert.True(t, rm.ManagedByThis())
	if !rm.ManagedByThis() {
		t.Error("RoleMapping has wrong ManagedBy value set")
		t.FailNow()
	}
}

func TestUpdateAwsAuthMapRoles(t *testing.T) {
	tm := metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}
	om := metav1.ObjectMeta{
		Name:      "aws-auth",
		Namespace: "kube-system",
	}
	cm := &v1.ConfigMap{
		TypeMeta:   tm,
		ObjectMeta: om,
		Immutable:  nil,
		Data: map[string]string{
			"mapRoles": `
    - groups:
      - system:masters
      rolearn: arn:aws:iam::1234:role/EKSAdmin
      username: system:node:eksadmin
    - groups:
      - system:masters
      rolearn: arn:aws:iam::9786:role/AWSReservedSSO_CapabilityAccess_dummy
      username: emcla-sandbox:sso-{{SessionName}}
    - groups:
      - DFDS-ReadOnly
      - ds-toolbox-deployment-rxpga
      rolearn: arn:aws:iam::0000:role/Capability
      username: sso:{{SessionName}}
`,
		},
		BinaryData: nil,
	}

	client := fake.NewSimpleClientset(cm)

	lrm, err := LoadAwsAuthMapRoles(client)
	assert.NoError(t, err)

	lrm.Mappings = append(lrm.Mappings, &RoleMapping{
		RoleARN:     "arn:aws:iam::9999:role/Capability",
		ManagedBy:   "aad-aws-sync",
		LastUpdated: "-1",
		CreatedAt:   "-1",
		Username:    "dummy",
		Groups:      []string{"DFDS-ReadOnly", "system:masters"},
	})

	payload, err := yaml.Marshal(&lrm.Mappings)
	assert.NoError(t, err)

	cm = &v1.ConfigMap{
		TypeMeta:   tm,
		ObjectMeta: om,
		Immutable:  nil,
		Data: map[string]string{
			"mapRoles": string(payload),
		},
		BinaryData: nil,
	}

	err = UpdateAwsAuthMapRoles(client, cm)
	assert.NoError(t, err)

	lrm, err = LoadAwsAuthMapRoles(client)
	assert.NoError(t, err)

	mapping := lrm.GetMappingByArn("arn:aws:iam::9999:role/Capability")

	assert.NotNil(t, mapping)

	assert.Equal(t, mapping.Username, "dummy")
	assert.Equal(t, mapping.Groups, []string{"DFDS-ReadOnly", "system:masters"})
}

//func Test_getK8sClient(t *testing.T) {
//	client, err := GetK8sClient()
//	assert.NoError(t, err)
//
//	assert.NotNil(t, client)
//}
