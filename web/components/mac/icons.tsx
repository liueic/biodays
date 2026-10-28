import type { SVGProps } from "react";

/**
 * Hand-drawn classic-Mac-style 1-bit icons (drawn for biodays, not Apple artwork).
 * Authored on a 16×16 pixel grid; render at 32×32 (desktop) or 16×16 (menus).
 */
type IconProps = SVGProps<SVGSVGElement>;

function Base({ children, ...props }: IconProps) {
  return (
    <svg
      viewBox="0 0 16 16"
      width={32}
      height={32}
      shapeRendering="crispEdges"
      aria-hidden
      {...props}
    >
      {children}
    </svg>
  );
}

export function MacIconDoc(props: IconProps) {
  return (
    <Base {...props}>
      <path d="M4 1h6l3 3v11H4z" fill="#fff" stroke="#000" />
      <path d="M10 1v3h3" fill="none" stroke="#000" />
      <path d="M6 7h5M6 9h5M6 11h5" fill="none" stroke="#000" />
    </Base>
  );
}

export function MacIconFolder(props: IconProps) {
  return (
    <Base {...props}>
      <path d="M1 4h5l2 2h7v8H1z" fill="#fff" stroke="#000" />
      <path d="M1 6h6" stroke="#000" />
    </Base>
  );
}

export function MacIconSparkle(props: IconProps) {
  return (
    <Base {...props}>
      <path d="M8 1l1.8 5.2L15 8l-5.2 1.8L8 15l-1.8-5.2L1 8l5.2-1.8z" fill="#fff" stroke="#000" />
    </Base>
  );
}

export function MacIconMail(props: IconProps) {
  return (
    <Base {...props}>
      <rect x="1" y="4" width="14" height="9" fill="#fff" stroke="#000" />
      <path d="M1 4l7 5.5L15 4" fill="none" stroke="#000" />
    </Base>
  );
}

export function MacIconHappyMac(props: IconProps) {
  return (
    <Base {...props}>
      <path d="M4 11h8l1.5 3.5h-11z" fill="#fff" stroke="#000" />
      <rect x="3" y="1" width="10" height="10" fill="#fff" stroke="#000" />
      <rect x="4.5" y="2.5" width="7" height="6" fill="#fff" stroke="#000" />
      <path d="M6.2 4.2v1.4M9.8 4.2v1.4M6.2 7q1.8 1.5 3.6 0" fill="none" stroke="#000" />
    </Base>
  );
}

export function MacIconBook(props: IconProps) {
  return (
    <Base {...props}>
      <path
        d="M8 4C6.5 2.8 4 2.5 2 3v10c2-.5 4.5-.2 6 1 1.5-1.2 4-1.5 6-1V3c-2-.5-4.5-.2-6 1z"
        fill="#fff"
        stroke="#000"
      />
      <path d="M8 4v9.5" stroke="#000" />
    </Base>
  );
}

/** Menu-bar Apple mark: a plain apple with a leaf, drawn from scratch. */
export function MacIconApple(props: IconProps) {
  return (
    <Base width={16} height={16} shapeRendering="auto" {...props}>
      <path
        d="M8 5.4C5.2 3.6 2.4 5.6 3.3 9.2 4 12 5.6 14.6 7 14.6c.6 0 .9-.4 1-.4s.4.4 1 .4c1.4 0 3-2.6 3.7-5.4.9-3.6-1.9-5.6-4.7-3.8z"
        fill="currentColor"
      />
      <path d="M8.2 4.8C8 2.8 9.4 1.2 11.6 1c.1 2.2-1.3 3.7-3.4 3.8z" fill="currentColor" />
    </Base>
  );
}

const APPLE_BODY =
  "M8 5.4C5.2 3.6 2.4 5.6 3.3 9.2 4 12 5.6 14.6 7 14.6c.6 0 .9-.4 1-.4s.4.4 1 .4c1.4 0 3-2.6 3.7-5.4.9-3.6-1.9-5.6-4.7-3.8z";

/** The rainbow Apple logo from the classic menu bar: six colored bands + green leaf. */
export function MacIconAppleColor(props: IconProps) {
  return (
    <Base width={16} height={16} shapeRendering="auto" {...props}>
      <defs>
        <clipPath id="mac-apple-clip">
          <path d={APPLE_BODY} />
        </clipPath>
      </defs>
      <g clipPath="url(#mac-apple-clip)">
        <rect x="0" y="3.4" width="16" height="1.95" fill="#61bb46" />
        <rect x="0" y="5.35" width="16" height="1.85" fill="#fdb827" />
        <rect x="0" y="7.2" width="16" height="1.85" fill="#f5821f" />
        <rect x="0" y="9.05" width="16" height="1.85" fill="#e03a3e" />
        <rect x="0" y="10.9" width="16" height="1.85" fill="#963d97" />
        <rect x="0" y="12.75" width="16" height="1.95" fill="#009ddc" />
      </g>
      <path d="M8.2 4.8C8 2.8 9.4 1.2 11.6 1c.1 2.2-1.3 3.7-3.4 3.8z" fill="#61bb46" />
    </Base>
  );
}

/** The classic Finder / "Mac OS" double face: blue profile on the left, white
    frontal face on the right. Sits at the right end of the menu bar. */
export function MacIconFinder(props: IconProps) {
  return (
    <Base width={16} height={16} shapeRendering="auto" {...props}>
      <rect x="1" y="1" width="14" height="14" rx="2" fill="#fff" stroke="#000" />
      <path d="M1 3a2 2 0 0 1 2-2h5v14H3a2 2 0 0 1-2-2z" fill="#3366cc" stroke="#000" />
      {/* profile face on the blue half: eye + nose + mouth */}
      <path d="M4.5 5.5v2M4 9.5l2 1-1.4 1.6" fill="none" stroke="#fff" />
      {/* frontal face on the white half: two eyes + smile */}
      <path d="M11 5v2.2M10 11.2q1.6 1.2 3 .2" fill="none" stroke="#000" />
    </Base>
  );
}

/** Mac OS 8 "Platinum" folder: lavender 3D folder, the classic desktop look. */
export function MacIconFolderColor(props: IconProps) {
  return (
    <Base {...props}>
      <path d="M1 4h5l2 2h7v8H1z" fill="#b9b9dd" stroke="#000" />
      <path d="M1 6h6.2L5.4 4.6" fill="none" stroke="#000" />
      <path d="M1.6 6.6h12.8l-1 6.9H2.6z" fill="#d5d5f2" stroke="#000" />
      <path d="M2.6 13.5h10.8l1-6.9" fill="none" stroke="#8a8ab8" />
    </Base>
  );
}

/** Classic hard-disk icon: gray 3D box with a front slot and activity light. */
export function MacIconHD(props: IconProps) {
  return (
    <Base {...props}>
      <path d="M3 3h10l2 3v7H1V6z" fill="#d4d4d4" stroke="#000" />
      <path d="M1 6h14M3 3l-2 3" fill="none" stroke="#000" />
      <path d="M3 11h7" stroke="#888" />
      <rect
        x="12"
        y="10.2"
        width="1.6"
        height="1.6"
        fill="#61bb46"
        stroke="#000"
        strokeWidth="0.6"
      />
    </Base>
  );
}

/** The Trash can: silver body with vertical ridges, a lid and a handle. */
export function MacIconTrash(props: IconProps) {
  return (
    <Base {...props}>
      <rect x="6.3" y="1" width="3.4" height="1.6" fill="#c8c8c8" stroke="#000" strokeWidth="0.8" />
      <rect x="3" y="2.8" width="10" height="1.8" fill="#d9d9d9" stroke="#000" />
      <path d="M4 4.6h8l-.7 9.4H4.7z" fill="#c8c8c8" stroke="#000" />
      <path d="M6 5.6v7.4M8 5.6v7.4M10 5.6v7.4" stroke="#888" />
    </Base>
  );
}

export function MacIconGlobe(props: IconProps) {
  return (
    <Base width={16} height={16} {...props}>
      <circle cx="8" cy="8" r="6.5" fill="#fff" stroke="#000" />
      <ellipse cx="8" cy="8" rx="3" ry="6.5" fill="none" stroke="#000" />
      <path d="M1.5 8h13M3 4.5h10M3 11.5h10" fill="none" stroke="#000" />
    </Base>
  );
}
