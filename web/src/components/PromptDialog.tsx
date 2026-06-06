import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface PromptOptions {
  title: string;
  description?: string;
  label?: string;
  placeholder?: string;
  defaultValue?: string;
  confirmLabel?: string;
  /** type of the input (text/password). */
  inputType?: "text" | "password";
}

type PromptFn = (opts: PromptOptions) => Promise<string | null>;

const PromptContext = createContext<PromptFn | null>(null);

/**
 * PromptProvider exposes an imperative prompt() that resolves to the entered
 * string (or null if cancelled), rendering a styled shadcn dialog instead of
 * the browser's prompt().
 */
export function PromptProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<PromptOptions | null>(null);
  const [value, setValue] = useState("");
  const resolverRef = useRef<((v: string | null) => void) | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const prompt = useCallback<PromptFn>((o) => {
    setOpts(o);
    setValue(o.defaultValue ?? "");
    return new Promise<string | null>((resolve) => {
      resolverRef.current = resolve;
    });
  }, []);

  useEffect(() => {
    if (opts) setTimeout(() => inputRef.current?.focus(), 50);
  }, [opts]);

  function close(result: string | null) {
    resolverRef.current?.(result);
    resolverRef.current = null;
    setOpts(null);
  }

  return (
    <PromptContext.Provider value={prompt}>
      {children}
      <Dialog open={!!opts} onOpenChange={(o) => !o && close(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{opts?.title}</DialogTitle>
            {opts?.description && <DialogDescription>{opts.description}</DialogDescription>}
          </DialogHeader>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (value.trim()) close(value);
            }}
            className="space-y-3"
          >
            {opts?.label && <Label htmlFor="prompt-input">{opts.label}</Label>}
            <Input
              id="prompt-input"
              ref={inputRef}
              type={opts?.inputType ?? "text"}
              placeholder={opts?.placeholder}
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => close(null)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!value.trim()}>
                {opts?.confirmLabel ?? "OK"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </PromptContext.Provider>
  );
}

export function usePrompt(): PromptFn {
  const ctx = useContext(PromptContext);
  if (!ctx) throw new Error("usePrompt must be used within PromptProvider");
  return ctx;
}
