import { cn } from "@/lib/utils";

/**
 * Logo is the UniDisk brand mark: three storage sources converging into a
 * single unified pool/disk. It's a single-color line mark using currentColor,
 * so it inherits the surrounding text color and themes automatically (just
 * like a Lucide icon). Size it with a className, e.g. <Logo className="h-5 w-5" />.
 */
export function Logo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={cn("h-6 w-6", className)}
      aria-hidden="true"
    >
      {/* source accounts */}
      <circle cx="5" cy="4" r="1.6" />
      <circle cx="12" cy="3" r="1.6" />
      <circle cx="19" cy="4" r="1.6" />
      {/* streams converging into the pool */}
      <path d="m6 5.4 4.4 4" />
      <path d="M12 4.6v4.8" />
      <path d="m18 5.4-4.4 4" />
      {/* unified pool (disk) */}
      <ellipse cx="12" cy="12.5" rx="8" ry="3" />
      <path d="M4 12.5V18c0 1.66 3.58 3 8 3s8-1.34 8-3v-5.5" />
    </svg>
  );
}
