import { useState } from 'react'
import { FileText } from './Lucide'

interface MarkdownPaneProps {
  worktreePath: string
}

const COMMON_MD_FILES = ['CLAUDE.md', 'AGENTS.md', 'BLOCKED.md', 'plan.md']

export default function MarkdownPane({ worktreePath }: MarkdownPaneProps) {
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [content, setContent] = useState('')

  const handleSelectFile = (filename: string) => {
    setSelectedFile(filename)
    // Placeholder: in the future, fetch file content via API or Tauri FS
    setContent(`# ${filename}\n\nFile content will be loaded from:\n${worktreePath}/${filename}\n\n(File reading not yet implemented)`)
  }

  return (
    <div className="flex h-full overflow-hidden">
      {/* File list */}
      <div className="w-40 border-r border-border bg-bg-secondary overflow-y-auto shrink-0">
        <div className="px-3 pt-3 pb-1 text-[10px] font-semibold tracking-widest text-text-secondary uppercase">
          Markdown Files
        </div>
        <div className="flex flex-col gap-0.5 px-1 py-1">
          {COMMON_MD_FILES.map((filename) => (
            <button
              key={filename}
              className={`flex items-center gap-2 w-full rounded px-2 py-1.5 text-left text-xs transition-colors ${
                selectedFile === filename
                  ? 'bg-accent/20 text-accent'
                  : 'text-text-primary hover:bg-bg-tertiary'
              }`}
              onClick={() => handleSelectFile(filename)}
              type="button"
            >
              <FileText size={12} />
              <span className="truncate">{filename}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Editor area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {selectedFile ? (
          <>
            <div className="flex items-center gap-2 px-3 py-2 border-b border-border bg-bg-secondary shrink-0">
              <FileText size={14} className="text-text-secondary" />
              <span className="text-sm text-text-primary">{selectedFile}</span>
              <span className="text-xs text-text-secondary">({worktreePath})</span>
            </div>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className="flex-1 resize-none bg-bg-primary p-4 text-sm text-text-primary font-mono leading-relaxed focus:outline-none"
              spellCheck={false}
            />
          </>
        ) : (
          <div className="flex items-center justify-center h-full text-text-secondary text-sm">
            Select a markdown file to view
          </div>
        )}
      </div>
    </div>
  )
}
