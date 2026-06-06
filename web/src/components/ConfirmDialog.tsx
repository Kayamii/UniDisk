import { createContext, useCallback, useContext, useState, type ReactNode } from "react";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface ConfirmOptions {
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
}

type ConfirmFn = (opts: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

/**
 * ConfirmProvider exposes an imperative confirm() that resolves to true/false,
 * rendering a styled shadcn dialog instead of the browser's confirm().
 */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const [resolver, setResolver] = useState<((v: boolean) => void) | null>(null);

  const confirm = useCallback<ConfirmFn>((o) => {
    setOpts(o);
    return new Promise<boolean>((resolve) => setResolver(() => resolve));
  }, []);

  function close(result: boolean) {
    resolver?.(result);
    setResolver(null);
    setOpts(null);
  }

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <Dialog open={!!opts} onOpenChange={(o) => !o && close(false)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{opts?.title}</DialogTitle>
            {opts?.description && <DialogDescription>{opts.description}</DialogDescription>}
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => close(false)}>
              {opts?.cancelLabel ?? "Cancel"}
            </Button>
            <Button
              variant={opts?.destructive ? "destructive" : "default"}
              onClick={() => close(true)}
            >
              {opts?.confirmLabel ?? "Confirm"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  );
}

export function useConfirm(): ConfirmFn {
  const ctx = useContext(ConfirmContext);
  if (!ctx) throw new Error("useConfirm must be used within ConfirmProvider");
  return ctx;
}
