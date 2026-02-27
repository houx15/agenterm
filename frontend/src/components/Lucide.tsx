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

export function ChevronDown({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <polyline points="6 9 12 15 18 9" />
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

export function ClipboardList({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect height="18" rx="2" width="14" x="5" y="3" />
      <line x1="9" x2="15" y1="9" y2="9" />
      <line x1="9" x2="15" y1="13" y2="13" />
      <line x1="9" x2="13" y1="17" y2="17" />
      <path d="M9 3.5h6v2H9z" />
    </IconBase>
  )
}

export function Smartphone({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect height="20" rx="2" width="12" x="6" y="2" />
      <line x1="12" x2="12.01" y1="18" y2="18" />
    </IconBase>
  )
}

export function Sun({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <circle cx="12" cy="12" r="4" />
      <line x1="12" x2="12" y1="2" y2="5" />
      <line x1="12" x2="12" y1="19" y2="22" />
      <line x1="2" x2="5" y1="12" y2="12" />
      <line x1="19" x2="22" y1="12" y2="12" />
      <line x1="4.9" x2="7.1" y1="4.9" y2="7.1" />
      <line x1="16.9" x2="19.1" y1="16.9" y2="19.1" />
      <line x1="16.9" x2="19.1" y1="7.1" y2="4.9" />
      <line x1="4.9" x2="7.1" y1="19.1" y2="16.9" />
    </IconBase>
  )
}

export function Moon({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M21 12.8A8.5 8.5 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
    </IconBase>
  )
}

export function PanelRight({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect width="18" height="18" x="3" y="3" rx="2" />
      <path d="M15 3v18" />
    </IconBase>
  )
}

export function PanelLeft({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect width="18" height="18" x="3" y="3" rx="2" />
      <path d="M9 3v18" />
    </IconBase>
  )
}

export function Bot({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M12 8V4H8" />
      <rect width="16" height="12" x="4" y="8" rx="2" />
      <path d="M2 14h2" />
      <path d="M20 14h2" />
      <path d="M15 13v2" />
      <path d="M9 13v2" />
    </IconBase>
  )
}

export function TerminalIcon({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M12 19h8" />
      <path d="m4 17 6-6-6-6" />
    </IconBase>
  )
}

export function Layers({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83z" />
      <path d="M2 12a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 12" />
      <path d="M2 17a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 17" />
    </IconBase>
  )
}

export function QrCode({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <rect width="5" height="5" x="3" y="3" rx="1" />
      <rect width="5" height="5" x="16" y="3" rx="1" />
      <rect width="5" height="5" x="3" y="16" rx="1" />
      <path d="M21 16h-3a2 2 0 0 0-2 2v3" />
      <path d="M21 21v.01" />
      <path d="M12 7v3a2 2 0 0 1-2 2H7" />
      <path d="M3 12h.01" />
      <path d="M12 3h.01" />
      <path d="M12 16v.01" />
      <path d="M16 12h1" />
      <path d="M21 12v.01" />
      <path d="M12 21v-1" />
    </IconBase>
  )
}

export function Inbox({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <polyline points="22 12 16 12 14 15 10 15 8 12 2 12" />
      <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
    </IconBase>
  )
}

export function X({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M18 6 6 18" />
      <path d="m6 6 12 12" />
    </IconBase>
  )
}

export function Maximize2({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M15 3h6v6" />
      <path d="m21 3-7 7" />
      <path d="m3 21 7-7" />
      <path d="M9 21H3v-6" />
    </IconBase>
  )
}

export function Minimize2({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="m14 10 7-7" />
      <path d="M20 10h-6V4" />
      <path d="m3 21 7-7" />
      <path d="M4 14h6v6" />
    </IconBase>
  )
}

export function Circle({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <circle cx="12" cy="12" r="10" />
    </IconBase>
  )
}

export function CircleDot({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <circle cx="12" cy="12" r="10" />
      <circle cx="12" cy="12" r="1" />
    </IconBase>
  )
}

export function ArrowRight({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M5 12h14" />
      <path d="m12 5 7 7-7 7" />
    </IconBase>
  )
}

export function Trash2({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M10 11v6" />
      <path d="M14 11v6" />
      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" />
      <path d="M3 6h18" />
      <path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
    </IconBase>
  )
}

export function Pencil({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z" />
      <path d="m15 5 4 4" />
    </IconBase>
  )
}

export function RefreshCw({ size, ...props }: IconProps) {
  return (
    <IconBase size={size} {...props}>
      <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
      <path d="M21 3v5h-5" />
      <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
      <path d="M8 16H3v5" />
    </IconBase>
  )
}
