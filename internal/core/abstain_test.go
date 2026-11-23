package core

import "testing"

func TestAbstainValid(t *testing.T) {
	cases := []struct {
		name string
		a    Abstain
		ok   bool
	}{
		{"空理由", Abstain{NotDone: []string{"x"}}, false},
		{"空未完成", Abstain{Reason: "无法验证"}, false},
		{"正常", Abstain{Reason: "环境无网络", Done: []string{"读文件"}, NotDone: []string{"跑测试"}, Next: "联网后重试"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.a.Valid()
			if tc.ok && err != nil {
				t.Fatalf("valid abstain rejected: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("invalid abstain accepted")
			}
		})
	}
}

func TestAbstainTerminal(t *testing.T) {
	a := Abstain{Reason: "r", NotDone: []string{"x"}}
	if a.Terminal() != TermUntest || a.Terminal().ExitCode() != 3 {
		t.Fatal("abstain must map to untested with exit code 3")
	}
	d := a.ToAbstainData()
	if d.Reason != "r" || len(d.NotDone) != 1 {
		t.Fatalf("data = %+v", d)
	}
}
