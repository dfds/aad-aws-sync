package handler

import (
	"strings"

	"go.dfds.cloud/aad-aws-sync/internal/capsvc"
)

// IgnoredEmailSuffix marks service-account addresses (e.g. "<name>.s@dfds.cloud")
// that must be entirely excluded from syncing: they are never added to or
// removed from Azure AD groups, application assignments, or Exchange email
// aliases.
const IgnoredEmailSuffix = ".s@dfds.cloud"

// ShouldIgnoreUser reports whether an email or UPN belongs to an account that
// must be entirely excluded from syncing.
func ShouldIgnoreUser(emailOrUpn string) bool {
	return strings.HasSuffix(strings.ToLower(emailOrUpn), IgnoredEmailSuffix)
}

// filterIgnoredMembers strips ignored members from every capability's member
// list, so downstream sync logic never attempts to use them. It is the single
// choke point for the "add"/"create" direction; the reverse (removal) loops
// must additionally guard with ShouldIgnoreUser so existing ignored accounts
// are left untouched rather than reconciled away.
func filterIgnoredMembers(capabilities []*capsvc.GetCapabilitiesResponseContextCapability) []*capsvc.GetCapabilitiesResponseContextCapability {
	for _, capability := range capabilities {
		kept := make([]capsvc.GetCapabilitiesResponseContextCapabilityMember, 0, len(capability.Members))
		for _, member := range capability.Members {
			if ShouldIgnoreUser(member.Email) {
				continue
			}
			kept = append(kept, member)
		}
		capability.Members = kept
	}
	return capabilities
}
