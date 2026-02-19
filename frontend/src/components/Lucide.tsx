import type { SVGProps } from 'react'

type IconProps = SVGProps<SVGSVGElement> & {
  size?: number
}

function IconBase({ size = 16, children, ...props }: IconProps) {
  return (
    <svg
      fill="none"
      height={size}
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="2"
      viewBox="0 0 24 24"
      width={size}
      {...props}
    >
      {children}
    </svg>
  )
}

export function Menu({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <line x1="4" x2="20" y1="6" y2="6" />
      <line x1="4" x2="20" y1="12" y2="12" />
      <line x1="4" x2="20" y1="18" y2="18" />
    </IconBase>
  )
}

export function House({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M3 10.5 12 3l9 7.5" />
      <path d="M5 9.8V20h14V9.8" />
      <path d="M10 20v-6h4v6" />
    </IconBase>
  )
}

export function SquareTerminal({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect height="18" rx="2" width="18" x="3" y="3" />
      <path d="m8 9 3 3-3 3" />
      <line x1="13" x2="17" y1="15" y2="15" />
    </IconBase>
  )
}

export function MessageSquareText({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M21 15a2 2 0 0 1-2 2H9l-4 4V5a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2z" />
      <line x1="9" x2="15" y1="9" y2="9" />
      <line x1="9" x2="17" y1="12" y2="12" />
    </IconBase>
  )
}

export function Settings({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1 1 0 0 0 .2 1.1l.1.1a2 2 0 0 1-2.8 2.8l-.1-.1a1 1 0 0 0-1.1-.2 1 1 0 0 0-.6.9V20a2 2 0 0 1-4 0v-.1a1 1 0 0 0-.6-.9 1 1 0 0 0-1.1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1 1 0 0 0 .2-1.1 1 1 0 0 0-.9-.6H4a2 2 0 0 1 0-4h.1a1 1 0 0 0 .9-.6 1 1 0 0 0-.2-1.1l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1 1 0 0 0 1.1.2 1 1 0 0 0 .6-.9V4a2 2 0 1 1 4 0v.1a1 1 0 0 0 .6.9 1 1 0 0 0 1.1-.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1 1 0 0 0-.2 1.1 1 1 0 0 0 .9.6h.1a2 2 0 0 1 0 4h-.1a1 1 0 0 0-.9.6z" />
    </IconBase>
  )
}

export function FolderPlus({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <line x1="12" x2="12" y1="11" y2="17" />
      <line x1="9" x2="15" y1="14" y2="14" />
    </IconBase>
  )
}

export function Plus({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <line x1="12" x2="12" y1="5" y2="19" />
      <line x1="5" x2="19" y1="12" y2="12" />
    </IconBase>
  )
}

export function Square({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect height="14" rx="2" width="14" x="5" y="5" />
    </IconBase>
  )
}

export function ChevronLeft({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <polyline points="15 18 9 12 15 6" />
    </IconBase>
  )
}

export function ChevronRight({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <polyline points="9 18 15 12 9 6" />
    </IconBase>
  )
}

export function FolderOpen({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M3 8a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v1H3z" />
      <path d="M3 11h18l-2 8a2 2 0 0 1-2 1H5a2 2 0 0 1-2-2z" />
    </IconBase>
  )
}

export function Activity({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
    </IconBase>
  )
}
