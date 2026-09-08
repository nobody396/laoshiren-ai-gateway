import claudeCodeGuide from '@/docs/simple/integration-claude-code.md?raw'
import codexGuide from '@/docs/simple/integration-codex.md?raw'
import grokBuildGuide from '@/docs/simple/integration-grok-build.md?raw'
import openCodeGuide from '@/docs/simple/integration-opencode.md?raw'
import antigravityGuide from '@/docs/simple/integration-antigravity.md?raw'
import kimiCodeGuide from '@/docs/simple/integration-kimi-code.md?raw'
import zcodeGuide from '@/docs/simple/integration-zcode.md?raw'
import workbuddyGuide from '@/docs/simple/integration-workbuddy.md?raw'

export interface SimpleClientGuide {
  id: string
  checkedVersion: string
  content: string
}

export const simpleClientGuides: readonly SimpleClientGuide[] = [
  { id: 'claude-code', checkedVersion: 'Claude Code 2.1.263', content: claudeCodeGuide },
  { id: 'codex', checkedVersion: 'Codex CLI 0.153.4', content: codexGuide },
  { id: 'grok-build', checkedVersion: 'Grok Build 1.0.13', content: grokBuildGuide },
  { id: 'opencode', checkedVersion: 'OpenCode 1.18.29', content: openCodeGuide },
  { id: 'antigravity', checkedVersion: 'Antigravity CLI 1.1.27', content: antigravityGuide },
  { id: 'kimi-code', checkedVersion: 'Kimi Code CLI 0.41.0', content: kimiCodeGuide },
  { id: 'zcode', checkedVersion: 'ZCode App 3.11.2', content: zcodeGuide },
  { id: 'workbuddy', checkedVersion: 'WorkBuddy 5.5.3', content: workbuddyGuide },
]

export const simpleClientGuideById = Object.fromEntries(
  simpleClientGuides.map(guide => [guide.id, guide]),
) as Readonly<Record<string, SimpleClientGuide>>
