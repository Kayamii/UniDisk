import { FlaskConical, Github, RotateCcw } from "lucide-react";
import { isDemo, resetDemo } from "@/lib/demo";
import { Button } from "@/components/ui/button";
import { useConfirm } from "@/components/ConfirmDialog";

const REPO_URL = "https://github.com/Kayamii/UniDisk";

/**
 * DemoBanner is rendered only in the public demo build. It states plainly that
 * nothing is stored server-side, and offers a reset back to the seeded state.
 */
export function DemoBanner() {
  const confirm = useConfirm();
  if (!isDemo) return null;

  async function reset() {
    const ok = await confirm({
      title: "Reset the demo?",
      description:
        "Discards every change you've made here and restores the sample data. Only affects this browser.",
      confirmLabel: "Reset",
      destructive: true,
    });
    if (!ok) return;
    await resetDemo();
    location.reload();
  }

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b bg-primary/5 px-4 py-2 text-xs sm:px-8">
      <span className="flex items-center gap-1.5 font-medium">
        <FlaskConical className="h-3.5 w-3.5 text-primary" />
        Live demo
      </span>
      <span className="text-muted-foreground">
        Runs entirely in your browser — no server, no account. Changes are saved
        to this browser's local storage and are visible only to you.
      </span>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={reset}>
          <RotateCcw className="h-3.5 w-3.5" />
          Reset demo
        </Button>
        <Button asChild variant="outline" size="sm" className="h-7 text-xs">
          <a href={REPO_URL} target="_blank" rel="noreferrer">
            <Github className="h-3.5 w-3.5" />
            Source
          </a>
        </Button>
      </div>
    </div>
  );
}
