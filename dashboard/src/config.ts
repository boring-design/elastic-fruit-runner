// REFRESH_INTERVAL_MS is the shared polling cadence for all dashboard data.
// It governs both the SWR refetch loop in useDashboardSync and the
// AUTO-REFRESH countdown rendered in the footer so the displayed value
// always matches the real schedule.
export const REFRESH_INTERVAL_MS = 5000
