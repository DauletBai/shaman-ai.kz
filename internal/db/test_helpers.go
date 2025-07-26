// internal/db/test_helpers.go
package db

import (
	"fmt"
	"shaman-ai.kz/internal/models"
	"testing"
)

func ClearTestDBTables(t *testing.T, tableNames ...string) {
	if DB == nil {
		t.Skip("DB not initialized, skipping table clear")
		return
	}
	for _, table := range tableNames {
		// For MariaDB/MySQL, TRUNCATE is faster but resets AUTO_INCREMENT. DELETE doesn't.
		// If FK constraints cause issues with TRUNCATE, use DELETE or temporarily disable FK checks.
		// For simplicity with FKs, DELETE is often safer in tests unless performance is critical.
		_, err := DB.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			// Attempt to TRUNCATE if DELETE fails due to FKs that should be cascaded but aren't set up for it.
			// Or, ensure your migrations set ON DELETE CASCADE where appropriate.
			// _, errTruncate := DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", table))
			// if errTruncate != nil {
			//  t.Logf("Failed to DELETE from table %s: %v. Also failed to TRUNCATE: %v", table, err, errTruncate)
			// }
			t.Fatalf("Failed to clear table %s: %v", table, err)
		}
		// Reset auto-increment if using DELETE and it matters for test predictability
		// This syntax is MySQL/MariaDB specific. For PostgreSQL use SETVAL.
		// _, err = DB.Exec(fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = 1", table))
		// if err != nil {
		//  t.Logf("Failed to reset auto_increment for table %s: %v", table, err)
		// }
	}
}

func SeedDefaultRolesForTest(t *testing.T) {
	if DB == nil {
		t.Skip("DB not initialized, skipping role seeding")
		return
	}
	rolesToSeed := []models.Role{
		{Name: models.RoleUser, Description: "Default user role"},
		{Name: models.RoleAdmin, Description: "Administrator role"},
	}
	for _, role := range rolesToSeed {
		_, err := CreateRoleIfNotExists(&role) // Assumes this function exists and handles "IF NOT EXISTS"
		if err != nil {
			t.Fatalf("Failed to seed role %s: %v", role.Name, err)
		}
	}
}