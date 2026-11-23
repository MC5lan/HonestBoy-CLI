package core

import "testing"

func gateEvidence(id string, kind EvidenceKind, target string, ok bool, p Provenance) Evidence {
	return Evidence{ID: id, Kind: kind, Target: target, OK: ok, Summary: "verifier output line",
		Level: EvidenceLevelVerifier, Ran: true, Provenance: p}
}

func TestGateSixCases(t *testing.T) {
	req := Requirement{Kinds: []EvidenceKind{EvBuild, EvTest}, Targets: []string{"./pkg"}, MinCount: 2}
	c := Claim{ID: "c1", Text: "完成", Req: req, Targets: []string{"./pkg"}}

	cases := []struct {
		name      string
		claim     Claim
		bag       EvidenceBag
		wantAllow bool
		wantRule  string
		wantMiss  int
	}{
		{
			name: "无证据申请完成", claim: c, bag: EvidenceBag{},
			wantAllow: false, wantRule: "coverage", wantMiss: 3, // target + 2 kinds
		},
		{
			name: "证据失败", claim: c,
			bag: bagOf(
				gateEvidence("e1", EvBuild, "./pkg", false, ProvObserved),
				gateEvidence("e2", EvTest, "./pkg", true, ProvObserved),
			),
			wantAllow: false, wantRule: "evidence_failed",
		},
		{
			name: "证据来源不足", claim: c,
			bag: bagOf(
				gateEvidence("e1", EvBuild, "./pkg", true, ProvInferred),
				gateEvidence("e2", EvTest, "./pkg", true, ProvObserved),
			),
			wantAllow: false, wantRule: "provenance",
		},
		{
			name: "只覆盖部分目标",
			claim: Claim{Req: Requirement{Kinds: []EvidenceKind{EvBuild}, Targets: []string{"./a", "./b"}},
				Targets: []string{"./a", "./b"}},
			bag:       bagOf(gateEvidence("e1", EvBuild, "./a", true, ProvObserved)),
			wantAllow: false, wantRule: "coverage", wantMiss: 1,
		},
		{
			name: "证据类型不符", claim: c,
			bag: bagOf(
				gateEvidence("e1", EvCommand, "./pkg", true, ProvObserved),
				gateEvidence("e2", EvCommand, "./pkg2", true, ProvObserved),
			),
			wantAllow: false, wantRule: "evidence_kind",
		},
		{
			// 探索过程中失败的 shell 命令不应阻断本次完成申请。
			name:  "无关失败证据不阻断",
			claim: Claim{Req: Requirement{Kinds: []EvidenceKind{EvDiff}, Targets: []string{"index.html"}}},
			bag: bagOf(
				gateEvidence("e1", EvDiff, "index.html", true, ProvObserved),
				gateEvidence("e2", EvCommand, "shell", false, ProvObserved),
			),
			wantAllow: true,
		},
		{
			// 参数错误导致的空目标失败 diff，不应覆盖后续成功的同文件写入。
			name:  "空目标失败证据不阻断",
			claim: Claim{Req: Requirement{Kinds: []EvidenceKind{EvDiff}, Targets: []string{"index.html"}}},
			bag: bagOf(
				gateEvidence("e1", EvDiff, "", false, ProvObserved),
				gateEvidence("e2", EvDiff, "index.html", true, ProvObserved),
			),
			wantAllow: true,
		},
		{
			// 目标上的错误证据类型仍然拒绝：不能拿命令证据冒充文件变更验证。
			name:  "目标上的错误类型仍拒绝",
			claim: Claim{Req: Requirement{Kinds: []EvidenceKind{EvDiff}, Targets: []string{"index.html"}}},
			bag: bagOf(
				gateEvidence("e1", EvCommand, "index.html", true, ProvObserved),
			),
			wantAllow: false, wantRule: "evidence_kind",
		},
		{
			name: "全部满足", claim: c,
			bag: bagOf(
				gateEvidence("e1", EvBuild, "./pkg", true, ProvObserved),
				gateEvidence("e2", EvTest, "./pkg", true, ProvObserved),
			),
			wantAllow: true,
		},
	}

	g := NewGate(Policy{Strict: true})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := g.Evaluate(tc.claim, tc.bag)
			if d.Allow != tc.wantAllow {
				t.Fatalf("Allow = %v, want %v (rule=%s reason=%s)", d.Allow, tc.wantAllow, d.Rule, d.Reason)
			}
			if !tc.wantAllow && d.Rule != tc.wantRule {
				t.Fatalf("Rule = %s, want %s", d.Rule, tc.wantRule)
			}
			if tc.wantMiss > 0 && len(d.Missing) < tc.wantMiss {
				t.Fatalf("Missing = %v, want at least %d", d.Missing, tc.wantMiss)
			}
			wantTerminal := TermDenied
			if tc.wantAllow {
				wantTerminal = TermDone
			}
			if d.Terminal != wantTerminal {
				t.Fatalf("Terminal = %s", d.Terminal)
			}
			if !tc.wantAllow && d.Reason == "" {
				t.Fatal("deny must carry a concrete reason")
			}
		})
	}
}

func TestGatePureTextAnswer(t *testing.T) {
	g := NewGate(Policy{Strict: false})
	d := g.Evaluate(Claim{Text: "解释一下 os.Root", Req: Requirement{NoEvidence: true}}, EvidenceBag{})
	if !d.Allow || d.Terminal != TermDone {
		t.Fatalf("pure text should allow done: %+v", d)
	}
	d = g.Evaluate(Claim{Text: "x"}, EvidenceBag{})
	if !d.Allow {
		t.Fatalf("claim without req/targets should allow done: %+v", d)
	}
}

func TestGateNeedFlags(t *testing.T) {
	g := NewGate(Policy{Strict: true})
	c := Claim{Req: Requirement{Kinds: []EvidenceKind{EvFile}, Targets: []string{"a.md"}, NeedReadEcho: true}}
	d := g.Evaluate(c, bagOf(gateEvidence("e1", EvFile, "a.md", true, ProvObserved)))
	if d.Allow || d.Rule != "coverage" {
		t.Fatalf("missing read_echo should deny: %+v", d)
	}
	c = Claim{Req: Requirement{Kinds: []EvidenceKind{EvTest}, Targets: []string{"a.go"}, NeedTestName: true}}
	d = g.Evaluate(c, bagOf(gateEvidence("e2", EvTest, "a.go", true, ProvObserved)))
	if d.Allow || d.Rule != "coverage" {
		t.Fatalf("missing test_name should deny: %+v", d)
	}
}

func bagOf(es ...Evidence) EvidenceBag {
	var b EvidenceBag
	for _, e := range es {
		b.Add(e)
	}
	return b
}
