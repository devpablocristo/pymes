package inventory

import (
	"testing"

	"github.com/google/uuid"
)

func TestStockLevelSelectBranchExpr(t *testing.T) {
	t.Run("without branch uses null placeholder", func(t *testing.T) {
		if got := stockLevelSelectBranchExpr(nil); got != "NULL AS branch_id" {
			t.Fatalf("stockLevelSelectBranchExpr(nil) = %q", got)
		}
	})

	t.Run("with branch uses joined branch column", func(t *testing.T) {
		branchID := uuid.New()
		if got := stockLevelSelectBranchExpr(&branchID); got != "sl.branch_id AS branch_id" {
			t.Fatalf("stockLevelSelectBranchExpr(branch) = %q", got)
		}
	})

	t.Run("with nil uuid uses null placeholder", func(t *testing.T) {
		nilUUID := uuid.Nil
		if got := stockLevelSelectBranchExpr(&nilUUID); got != "NULL AS branch_id" {
			t.Fatalf("stockLevelSelectBranchExpr(uuid.Nil) = %q", got)
		}
	})
}
