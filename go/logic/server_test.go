package logic

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"github.com/github/gh-ost/go/base"
	"github.com/stretchr/testify/require"
)

func TestServerRunCPUProfile(t *testing.T) {
	t.Parallel()

	t.Run("failed already running", func(t *testing.T) {
		s := &Server{isCPUProfiling: 1}
		profile, err := s.runCPUProfile("15ms")
		require.Equal(t, err, ErrCPUProfilingInProgress)
		require.Nil(t, profile)
	})

	t.Run("failed bad duration", func(t *testing.T) {
		s := &Server{isCPUProfiling: 0}
		profile, err := s.runCPUProfile("should-fail")
		require.Error(t, err)
		require.Nil(t, profile)
	})

	t.Run("failed bad option", func(t *testing.T) {
		s := &Server{isCPUProfiling: 0}
		profile, err := s.runCPUProfile("10ms,badoption")
		require.Equal(t, err, ErrCPUProfilingBadOption)
		require.Nil(t, profile)
	})

	t.Run("success", func(t *testing.T) {
		s := &Server{
			isCPUProfiling:   0,
			migrationContext: base.NewMigrationContext(),
		}
		defaultCPUProfileDuration = time.Millisecond * 10
		profile, err := s.runCPUProfile("")
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Equal(t, int64(0), s.isCPUProfiling)
	})

	t.Run("success with block", func(t *testing.T) {
		s := &Server{
			isCPUProfiling:   0,
			migrationContext: base.NewMigrationContext(),
		}
		profile, err := s.runCPUProfile("10ms,block")
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Equal(t, int64(0), s.isCPUProfiling)
	})

	t.Run("success with block and gzip", func(t *testing.T) {
		s := &Server{
			isCPUProfiling:   0,
			migrationContext: base.NewMigrationContext(),
		}
		profile, err := s.runCPUProfile("10ms,block,gzip")
		require.NoError(t, err)
		require.NotNil(t, profile)
		require.Equal(t, int64(0), s.isCPUProfiling)
	})
}

func TestServerPostponeCommand(t *testing.T) {
	migrationContext := base.NewMigrationContext()
	migrationContext.OriginalTableName = "test_table"
	hooksExecutor := NewHooksExecutor(migrationContext)
	server := &Server{
		migrationContext: migrationContext,
		hooksExecutor:    hooksExecutor,
	}

	tests := []struct {
		name           string
		command        string
		setupFlags     func()
		expectedOutput string
		expectError    bool
	}{
		{
			name:    "postpone when not postponed",
			command: "postpone",
			setupFlags: func() {
				// Reset all flags to simulate normal operation
				migrationContext.IsPostponingCutOver = 0
				migrationContext.CutOverCompleteFlag = 0
				migrationContext.InCutOverCriticalSectionFlag = 0
			},
			expectedOutput: "Postponed",
			expectError:    false,
		},
		{
			name:    "postpone when already postponed",
			command: "postpone",
			setupFlags: func() {
				migrationContext.IsPostponingCutOver = 1
			},
			expectedOutput: "Migration is already postponed.",
			expectError:    false,
		},
		{
			name:    "postpone when cut-over completed",
			command: "postpone",
			setupFlags: func() {
				migrationContext.IsPostponingCutOver = 0
				migrationContext.CutOverCompleteFlag = 1
			},
			expectedOutput: "Cut-over already completed. Cannot postpone.",
			expectError:    false,
		},
		{
			name:    "postpone during critical section",
			command: "postpone",
			setupFlags: func() {
				migrationContext.IsPostponingCutOver = 0
				migrationContext.CutOverCompleteFlag = 0
				migrationContext.InCutOverCriticalSectionFlag = 1
			},
			expectedOutput: "Cut-over is in critical section. Cannot postpone at this time.",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFlags()

			var output strings.Builder
			writer := bufio.NewWriter(&output)

			rule, err := server.applyServerCommand(tt.command, writer)
			writer.Flush()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !strings.Contains(output.String(), tt.expectedOutput) {
				t.Errorf("Expected output to contain '%s', got '%s'", tt.expectedOutput, output.String())
			}

			if tt.expectedOutput == "Postponed" && rule != ForcePrintStatusAndHintRule {
				t.Errorf("Expected ForcePrintStatusAndHintRule when postpone succeeds")
			}
		})
	}
}
