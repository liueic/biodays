"use client";

type MacTitleBarProps = {
  title: string;
  shaded?: boolean;
  maximized?: boolean;
  onClose?: () => void;
  onZoom?: () => void;
  onToggleShade?: () => void;
};

/** Title bar extracted from the reference MacWindow; the shared body stays mounted. */
export function MacTitleBar({
  title,
  shaded,
  maximized,
  onClose,
  onZoom,
  onToggleShade,
}: MacTitleBarProps) {
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: the collapse button exposes the same action to keyboard users.
    <header
      className="mac-titlebar window-drag-handle"
      data-os-only="mac"
      onDoubleClick={onToggleShade}
    >
      {onClose && (
        <button
          type="button"
          className="mac-titlebox mac-closebox"
          aria-label="关闭"
          onClick={onClose}
        />
      )}
      <span className="mac-titlebar-stripes" aria-hidden />
      <span className="mac-titlebar-title">{title}</span>
      <span className="mac-titlebar-stripes" aria-hidden />
      {onZoom && (
        <button
          type="button"
          className="mac-titlebox mac-zoombox"
          aria-label={maximized ? "还原" : "缩放"}
          onClick={onZoom}
        />
      )}
      {onToggleShade && (
        <button
          type="button"
          className="mac-titlebox mac-collapsebox"
          aria-label={shaded ? "展开" : "折叠"}
          aria-pressed={shaded}
          onClick={onToggleShade}
        />
      )}
    </header>
  );
}
