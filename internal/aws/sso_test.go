package aws

import (
	identityStoreTypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	orgTypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestManageSso_GetAccountById(t *testing.T) {
	mSso := ManageSso{
		awsAccountsById:    map[string]*orgTypes.Account{},
		awsAccountsByAlias: map[string]*orgTypes.Account{},
		awsSsoGroupById:    map[string]*identityStoreTypes.Group{},
		awsSsoGroupByName:  map[string]*identityStoreTypes.Group{},
	}
	acc := mSso.GetAccountById("9999")
	assert.Nil(t, acc)

	mSso.awsAccountsById["9999"] = &orgTypes.Account{}

	acc = mSso.GetAccountById("9999")
	assert.NotNil(t, acc)
}

func TestManageSso_GetAccountByName(t *testing.T) {
	mSso := ManageSso{
		awsAccountsById:    map[string]*orgTypes.Account{},
		awsAccountsByAlias: map[string]*orgTypes.Account{},
		awsSsoGroupById:    map[string]*identityStoreTypes.Group{},
		awsSsoGroupByName:  map[string]*identityStoreTypes.Group{},
	}
	acc := mSso.GetAccountByName("9999")
	assert.Nil(t, acc)

	mSso.awsAccountsByAlias["9999"] = &orgTypes.Account{}

	acc = mSso.GetAccountByName("9999")
	assert.NotNil(t, acc)
}

func TestManageSso_GetGroupById(t *testing.T) {
	mSso := ManageSso{
		awsAccountsById:    map[string]*orgTypes.Account{},
		awsAccountsByAlias: map[string]*orgTypes.Account{},
		awsSsoGroupById:    map[string]*identityStoreTypes.Group{},
		awsSsoGroupByName:  map[string]*identityStoreTypes.Group{},
	}
	grp := mSso.GetGroupById("9999")
	assert.Nil(t, grp)

	mSso.awsSsoGroupById["9999"] = &identityStoreTypes.Group{}

	grp = mSso.GetGroupById("9999")
	assert.NotNil(t, grp)
}

func TestManageSso_GetGroupByName(t *testing.T) {
	mSso := ManageSso{
		awsAccountsById:    map[string]*orgTypes.Account{},
		awsAccountsByAlias: map[string]*orgTypes.Account{},
		awsSsoGroupById:    map[string]*identityStoreTypes.Group{},
		awsSsoGroupByName:  map[string]*identityStoreTypes.Group{},
	}
	grp := mSso.GetGroupByName("9999")
	assert.Nil(t, grp)

	mSso.awsSsoGroupByName["9999"] = &identityStoreTypes.Group{}

	grp = mSso.GetGroupByName("9999")
	assert.NotNil(t, grp)
}

func TestRemoveAccountPrefix(t *testing.T) {
	val := RemoveAccountPrefix("dfds-", "dfds-test-account-1234")
	assert.Equal(t, val, "test-account-1234")
}
