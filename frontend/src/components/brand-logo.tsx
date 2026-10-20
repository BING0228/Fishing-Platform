import { cn } from "@/lib/utils";
import { SPEARHOOK_MARK_DATA_URL } from "./spearhook-mark";

type BrandLogoProps = {
  className?: string;
  size?: number;
  /** Sidebar uses white fill; solid mask fill when not colored/framed. */
  fill?: string;
  title?: string;
  /** Rounded app-icon treatment (favicon / README). */
  framed?: boolean;
  /** Theme slate hook + muted-gray spear color mark (login). */
  colored?: boolean;
};

/**
 * Spearhook mark: S-shaped fishhook with a diagonal spear through it.
 * Default: alpha mask so `fill` / currentColor recolor the glyph.
 * `colored`: primary slate hook + muted-foreground spear (theme palette).
 * `framed`: dark plate + metallic app-icon artwork.
 */
export function BrandLogo({
  className,
  size = 22,
  fill = "currentColor",
  title = "Spearhook",
  framed = false,
  colored = false,
}: BrandLogoProps) {
  const basePath = process.env.NEXT_PUBLIC_BASE_PATH || "";

  if (framed) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={`${basePath}/favicon-mark.png`}
        alt={title}
        title={title}
        width={size}
        height={size}
        className={cn("inline-block shrink-0 rounded-[22%] object-cover", className)}
        draggable={false}
      />
    );
  }

  if (colored) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={`${basePath}/brand-mark-color.png`}
        alt={title}
        title={title}
        width={size}
        height={size}
        className={cn("inline-block shrink-0 object-contain", className)}
        draggable={false}
      />
    );
  }

  return (
    <span
      role="img"
      aria-label={title}
      title={title}
      className={cn("inline-block shrink-0", className)}
      style={{
        width: size,
        height: size,
        backgroundColor: fill,
        WebkitMaskImage: `url(${SPEARHOOK_MARK_DATA_URL})`,
        maskImage: `url(${SPEARHOOK_MARK_DATA_URL})`,
        WebkitMaskSize: "contain",
        maskSize: "contain",
        WebkitMaskRepeat: "no-repeat",
        maskRepeat: "no-repeat",
        WebkitMaskPosition: "center",
        maskPosition: "center",
      }}
    />
  );
}
