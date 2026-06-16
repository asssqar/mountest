import type { HTMLAttributes, ReactNode } from "react";

type Props = HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
};

/** Блокирует выделение и копирование текста (не защита от скриншотов). */
export default function NoCopy({ children, className = "", ...rest }: Props) {
  return (
    <div
      {...rest}
      className={`no-copy select-none [&_img]:pointer-events-none [&_img]:select-none ${className}`}
      onCopy={(e) => e.preventDefault()}
      onCut={(e) => e.preventDefault()}
      onContextMenu={(e) => e.preventDefault()}
      onDragStart={(e) => e.preventDefault()}
    >
      {children}
    </div>
  );
}
