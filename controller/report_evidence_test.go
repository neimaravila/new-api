package controller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

// TestCaptureReportEndpointEvidence writes real admin/user report payloads to the
// goal scratch dir as durable evidence of the role-scoped endpoint behavior.
// The payloads are produced by the same service.GetReportSummary function the
// /api/dashboard/report handler invokes.
func TestCaptureReportEndpointEvidence(t *testing.T) {
	scratch := os.Getenv("GROAL_SCRATCH")
	setupReportControllerTestDB(t)

	adminSummary, err := service.GetReportSummary(99, "admin", common.RoleAdminUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)
	require.Equal(t, dto.DashboardRoleAdmin, adminSummary.Role)
	require.NotEmpty(t, adminSummary.Channels, "admin payload must include channel breakdown")

	userSummary, err := service.GetReportSummary(1, "alice", common.RoleCommonUser, dto.ReportRangeLast7Days, dto.ReportGranularityHour)
	require.NoError(t, err)
	require.Equal(t, dto.DashboardRoleUser, userSummary.Role)
	require.Empty(t, userSummary.Channels, "user payload must not include admin-only channel breakdown")

	// Evidence files are only written when GROAL_SCRATCH points at an existing dir.
	if scratch != "" {
		if info, err := os.Stat(scratch); err == nil && info.IsDir() {
			require.NoError(t, os.WriteFile(filepath.Join(scratch, "endpoints-admin.json"), []byte(common.GetJsonString(adminSummary)), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(scratch, "endpoints-user.json"), []byte(common.GetJsonString(userSummary)), 0o644))
		}
	}
}
