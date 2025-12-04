package variables

import (
	"strings"
	"testing"
)

// FuzzProcess tests variable processing with random inputs
func FuzzProcess(f *testing.F) {
	seeds := []string{
		"{{baseUrl}}/users",
		"{{$guid}}",
		"{{$timestamp}}",
		"{{$randomInt 1 100}}",
		"{{$datetime iso8601}}",
		"{{$datetime rfc1123}}",
		"https://api.example.com/users",
		"{{%encodedVar}}",
		"{{nested.value}}",
		"{{request.response.body.$.token}}",
		"",
		"{{}}",
		"{{  }}",
		"{{ var }}",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		vp := NewVariableProcessor()
		vp.SetFileVariables(map[string]string{
			"baseUrl":    "https://api.example.com",
			"token":      "test-token",
			"encodedVar": "hello world",
		})

		// Should not panic
		_, _ = vp.Process(input)
	})
}

// FuzzParseFileVariables tests file variable parsing
func FuzzParseFileVariables(f *testing.F) {
	seeds := []string{
		"@baseUrl = https://api.example.com",
		"@token = abc123\n@version = v1",
		"  @name   =   John Doe  ",
		"@baseUrl = https://api.example.com\n# comment\nGET {{baseUrl}}/users",
		"GET https://api.example.com",
		"@message = Hello\\nWorld",
		"",
		"@",
		"@ = ",
		"@=value",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ParseFileVariables(input)

		// Result should be a valid map
		if result == nil {
			t.Error("ParseFileVariables returned nil")
		}
	})
}

// FuzzParseFileVariablesWithDuplicates tests duplicate detection
func FuzzParseFileVariablesWithDuplicates(f *testing.F) {
	seeds := []string{
		"@baseUrl = https://api.example.com",
		"@baseUrl = https://api.example.com\n@baseUrl = https://staging.example.com",
		"@a = 1\n@b = 2\n@a = 3\n@b = 4",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ParseFileVariablesWithDuplicates(input)

		// Result should be valid
		if result.Variables == nil {
			t.Error("ParseFileVariablesWithDuplicates returned nil Variables")
		}
		// Note: Duplicates can be nil when there are no duplicates - this is valid Go behavior
	})
}

// FuzzExtractJSONPath tests JSON path extraction
func FuzzExtractJSONPath(f *testing.F) {
	jsonBody := `{"user": {"name": "John", "age": 30}, "items": ["a", "b"], "active": true}`

	seeds := []string{
		"$.user",
		"$.user.name",
		"$.items[0]",
		"$.active",
		"$",
		"",
		"$.nonexistent",
		"$.user.nonexistent",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, path string) {
		// Should not panic
		_, _ = extractJSONPath(jsonBody, path)
	})
}

// FuzzURLEncode tests URL encoding
func FuzzURLEncode(f *testing.F) {
	seeds := []string{
		"hello world",
		"a+b",
		"test@example.com",
		"key=value",
		"safe-string_123",
		"",
		"特殊字符",
		"emoji 🎉",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := urlEncode(input)

		// Result should not contain spaces (should be encoded)
		if strings.Contains(result, " ") {
			t.Error("urlEncode result contains unencoded space")
		}
	})
}

// FuzzConvertDateFormat tests date format conversion
func FuzzConvertDateFormat(f *testing.F) {
	seeds := []string{
		"YYYY-MM-DD",
		"YY/MM/DD",
		"YYYY-MM-DD HH:mm:ss",
		"'YYYY-MM-DD'",
		"HH:mm:ss",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = convertDateFormat(input)
	})
}

// FuzzProcessEscapes tests escape sequence processing
func FuzzProcessEscapes(f *testing.F) {
	seeds := []string{
		`Hello\nWorld`,
		`Tab\there`,
		`Return\rhere`,
		`Quote\"here`,
		`No escapes`,
		`Multiple\n\t\rescapes`,
		"",
		`\\`,
		`\`,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = processEscapes(input)
	})
}

// FuzzGenerateGUID tests GUID generation consistency
func FuzzGenerateGUID(f *testing.F) {
	// No real input for this, just test it doesn't panic with many calls
	f.Add(0)
	f.Add(1)
	f.Add(100)

	f.Fuzz(func(t *testing.T, seed int) {
		// Should not panic
		guid := GenerateGUID()

		// Should be in UUID format (36 chars with hyphens)
		if len(guid) != 36 {
			t.Errorf("GUID length = %d, want 36", len(guid))
		}

		// Should have hyphens in the right places
		if guid[8] != '-' || guid[13] != '-' || guid[18] != '-' || guid[23] != '-' {
			t.Errorf("GUID format invalid: %s", guid)
		}
	})
}

// FuzzNestedVariables tests nested variable resolution
func FuzzNestedVariables(f *testing.F) {
	seeds := []string{
		"{{baseUrl}}/{{version}}/users",
		"{{outer{{inner}}}}",
		"{{a}}{{b}}{{c}}",
		"prefix{{var}}suffix",
		"{{}}",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		vp := NewVariableProcessor()
		vp.SetFileVariables(map[string]string{
			"baseUrl": "https://api.example.com",
			"version": "v1",
			"a":       "1",
			"b":       "2",
			"c":       "3",
			"inner":   "nested",
			"outer":   "value",
			"var":     "variable",
		})

		// Should not panic
		_, _ = vp.Process(input)
	})
}

// FuzzAddDuration tests duration addition
func FuzzAddDuration(f *testing.F) {
	seeds := []struct {
		offset int
		unit   string
	}{
		{1, "y"},
		{-1, "y"},
		{1, "M"},
		{1, "w"},
		{1, "d"},
		{1, "h"},
		{1, "m"},
		{1, "s"},
		{1, "ms"},
		{0, "d"},
		{100, "d"},
		{-100, "d"},
	}

	for _, seed := range seeds {
		f.Add(seed.offset, seed.unit)
	}

	f.Fuzz(func(t *testing.T, offset int, unit string) {
		// Should not panic
		_ = addDuration(parseTime("2024-01-15T12:00:00Z"), offset, unit)
	})
}

// FuzzSystemVariables tests system variable resolution
func FuzzSystemVariables(f *testing.F) {
	seeds := []string{
		"$guid",
		"$timestamp",
		"$randomInt",
		"$datetime",
		"$localDatetime",
		"$processEnv",
		"$dotenv",
		"$prompt",
		"$unknown",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		vp := NewVariableProcessor()

		// Should not panic when processing system variables
		_, _ = vp.Process("{{" + input + "}}")
	})
}

// FuzzEnvironmentVariables tests environment variable handling
func FuzzEnvironmentVariables(f *testing.F) {
	seeds := []string{
		"dev",
		"prod",
		"staging",
		"$shared",
		"",
		"nonexistent",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, env string) {
		vp := NewVariableProcessor()
		vp.SetEnvironment(env)
		vp.SetEnvironmentVariables(map[string]map[string]string{
			"$shared": {"version": "v1"},
			"dev":     {"host": "dev.example.com"},
			"prod":    {"host": "example.com"},
		})

		// Should not panic
		_, _ = vp.Process("{{host}}")
		_, _ = vp.Process("{{version}}")
	})
}

// FuzzRequestResult tests request result variable resolution
func FuzzRequestResult(f *testing.F) {
	seeds := []string{
		"{{api.response.body.$.token}}",
		"{{api.response.headers.Content-Type}}",
		"{{api.response.body.*}}",
		"{{nonexistent.response.body.$.field}}",
		"{{api.invalid.path}}",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		vp := NewVariableProcessor()
		vp.SetRequestResult("api", RequestResult{
			StatusCode: 200,
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
			Body: `{"token": "abc123", "user": {"id": 1}}`,
		})

		// Should not panic
		_, _ = vp.Process(input)
	})
}
