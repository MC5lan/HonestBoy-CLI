package engine

import (
	"fmt"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/MC5lan/HonestBoy-CLI/internal/git"
)

// prepareUserChanges 在回合开始前把用户已有脏改动单独提交，避免混进 agent 的提交（Aider 做法）。
func (e *Engine) prepareUserChanges() {
	repo, err := git.Open(e.cfg.Workdir)
	if err != nil {
		return
	}
	dirty, err := repo.IsDirty()
	if err != nil || !dirty {
		return
	}
	files, _ := repo.ChangedFiles()
	if sha, err := repo.Commit("hb: snapshot user changes before turn"); err == nil {
		e.emit(core.EvtNotice, core.NoticeData{
			Level: "info", Text: fmt.Sprintf("用户已有改动已单独提交 %s（%d 个文件）", sha, len(files)),
		})
	}
}

// autoCommit 提交本回合的写操作，并发 diff_review 事件供 UI/日志审阅。
func (e *Engine) autoCommit(prompt string) {
	repo, err := git.Open(e.cfg.Workdir)
	if err != nil {
		return
	}
	stat, _ := repo.DiffStat()
	files, _ := repo.ChangedFiles()
	message := "hb: " + commitSubject(prompt)
	sha, err := repo.Commit(message)
	if err != nil {
		e.emit(core.EvtNotice, core.NoticeData{Level: "warn", Text: "自动提交失败: " + err.Error()})
		return
	}
	e.emit(core.EvtDiffReview, core.DiffReviewData{Commit: sha, Files: files, Stat: stat})
}

// commitSubject 从任务描述生成短提交主题（确定性，不调用模型）。
func commitSubject(prompt string) string {
	s := strings.TrimSpace(strings.ReplaceAll(prompt, "\n", " "))
	r := []rune(s)
	if len(r) > 50 {
		r = r[:50]
	}
	if len(r) == 0 {
		return "update"
	}
	return strings.TrimSpace(string(r))
}
