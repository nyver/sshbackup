package domain

import "testing"

func TestValidateName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"non-empty", "beresta", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateName("job", tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAbsoluteRemotePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"absolute", "/docker/volumes", false},
		{"relative", "docker/volumes", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateAbsoluteRemotePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAbsoluteRemotePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"min valid", 1, false},
		{"max valid", 65535, false},
		{"typical", 22, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too large", 65536, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort(%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCronExpression(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"every day at 3am", "0 3 * * *", false},
		{"every 15 minutes", "*/15 * * * *", false},
		{"empty", "", true},
		{"too many fields", "0 3 * * * *", true},
		{"garbage", "not a cron expression", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCronExpression(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCronExpression(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimeoutSeconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		seconds int
		wantErr bool
	}{
		{"positive", 300, false},
		{"zero", 0, true},
		{"negative", -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTimeoutSeconds(tt.seconds)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimeoutSeconds(%d) error = %v, wantErr %v", tt.seconds, err, tt.wantErr)
			}
		})
	}
}
