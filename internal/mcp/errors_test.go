package mcp

import (
	"testing"
)

func TestErrorAnalyzerHandlers(t *testing.T) {
	analyzer := NewErrorAnalyzer("config.yaml")

	tests := []struct {
		name    string
		handler func(map[string]interface{}) *StructuredError
		context map[string]interface{}
		code    ErrorCode
	}{
		{
			name:    "ProfileNotFound",
			handler: analyzer.handleProfileNotFound,
			context: map[string]interface{}{"profile_name": "missing"},
			code:    ErrorCodeProfileNotFound,
		},
		{
			name:    "ColumnNotFound",
			handler: analyzer.handleColumnNotFound,
			context: map[string]interface{}{"error": "column 'age' not found", "profile_name": "test", "table_name": "users"},
			code:    ErrorCodeColumnNotFound,
		},
		{
			name:    "ReadOnlyViolation",
			handler: analyzer.handleReadOnlyViolation,
			context: map[string]interface{}{"sql": "DROP TABLE users", "profile_name": "readonly_db"},
			code:    ErrorCodeReadOnlyViolation,
		},
		{
			name:    "PermissionDenied",
			handler: analyzer.handlePermissionDenied,
			context: map[string]interface{}{"error": "permission denied for table users"},
			code:    ErrorCodePermissionDenied,
		},
		{
			name:    "ConstraintViolation",
			handler: analyzer.handleConstraintViolation,
			context: map[string]interface{}{"error": "foreign key constraint fails"},
			code:    ErrorCodeConstraintViolation,
		},
		{
			name:    "DatabaseNotFound",
			handler: analyzer.handleDatabaseNotFound,
			context: map[string]interface{}{"database_name": "missing_db", "profile_name": "test"},
			code:    ErrorCodeDatabaseNotFound,
		},
		{
			name:    "DataTypeMismatch",
			handler: analyzer.handleDataTypeMismatch,
			context: map[string]interface{}{"error": "type mismatch", "table_name": "users", "profile_name": "test"},
			code:    ErrorCodeDataTypeMismatch,
		},
		{
			name:    "DecryptionFailed",
			handler: analyzer.handleDecryptionFailed,
			context: map[string]interface{}{"profile_name": "encrypted_db"},
			code:    ErrorCodeDecryptionFailed,
		},
		{
			name:    "UnsupportedDatabase",
			handler: analyzer.handleUnsupportedDatabase,
			context: map[string]interface{}{"db_type": "oracle"},
			code:    ErrorCodeUnsupportedDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.handler(tt.context)
			if result == nil {
				t.Fatalf("Expected non-nil error result")
			}
			if result.ErrorCode != tt.code {
				t.Fatalf("Expected code %s, got %s", tt.code, result.ErrorCode)
			}
			if len(result.Suggestions) == 0 {
				t.Fatalf("Expected suggestions")
			}
		})
	}
}
