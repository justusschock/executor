package local

import "testing"

// FR-EXEC-11
func TestWindowsShellCommandLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		arguments []string
		want      string
		ok        bool
	}{
		{
			name:      "default shell keeps quotes verbatim",
			path:      "cmd.exe",
			arguments: []string{"/d", "/s", "/c", `python train.py --name "my run" & echo \done`},
			want:      `cmd.exe /d /s /c "python train.py --name "my run" & echo \done"`,
			ok:        true,
		},
		{
			name:      "absolute path and uppercase flags",
			path:      `C:\Windows\System32\CMD.EXE`,
			arguments: []string{"/S", "/C", "dir"},
			want:      `C:\Windows\System32\CMD.EXE /S /C "dir"`,
			ok:        true,
		},
		{name: "without /s", path: "cmd.exe", arguments: []string{"/c", "dir"}},
		{name: "without /c before script", path: "cmd.exe", arguments: []string{"/s", "/k", "dir"}},
		{name: "other program", path: "powershell.exe", arguments: []string{"/s", "/c", "dir"}},
		{name: "path with spaces", path: `C:\Program Files\cmd.exe`, arguments: []string{"/s", "/c", "dir"}},
		{name: "flag with spaces", path: "cmd.exe", arguments: []string{"/s", "/t: 0a", "/c", "dir"}},
		{name: "script only", path: "cmd.exe", arguments: []string{"dir"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := windowsShellCommandLine(test.path, test.arguments)
			if got != test.want || ok != test.ok {
				t.Fatalf("windowsShellCommandLine() = (%q, %t), want (%q, %t)", got, ok, test.want, test.ok)
			}
		})
	}
}
