package postgres

import "testing"

// TestOperationName checks the labels derived from the repository's queries.
// TestOperationName 校验从 repository 实际 SQL 派生出的标签。
func TestOperationName(t *testing.T) {
	cases := []struct {
		sql  string
		want string
	}{
		{`INSERT INTO accounts (id, balance) VALUES ($1, $2)`, "insert accounts"},
		{`SELECT id, balance FROM accounts WHERE id = $1`, "select accounts"},
		{`SELECT id, balance FROM accounts WHERE id = ANY($1) ORDER BY id FOR UPDATE`, "select accounts (locked)"},
		{`UPDATE accounts SET balance = $2 WHERE id = $1`, "update accounts"},
		{`SELECT id FROM transfers WHERE idempotency_key = $1::uuid`, "select transfers"},
		{`INSERT INTO ledger_entries (transfer_id, account_id) VALUES ($1, $2)`, "insert ledger_entries"},
		{`SELECT pg_advisory_lock($1)`, "select"},
		{``, "unknown"},
	}
	for _, tc := range cases {
		if got := operationName(tc.sql); got != tc.want {
			t.Errorf("operationName(%q) = %q, want %q", tc.sql, got, tc.want)
		}
	}
}
