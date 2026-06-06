import { useEffect, useState } from "react";
import { Toaster as Sonner } from "sonner";

/**
 * Toaster renders app-wide toast notifications. It mirrors UniDisk's theme by
 * reading the `.light` class our AppLayout toggles on <html>, so toasts match
 * the current light/dark mode without an extra theme library.
 */
export function Toaster() {
  const [light, setLight] = useState(
    typeof document !== "undefined" && document.documentElement.classList.contains("light")
  );

  useEffect(() => {
    const el = document.documentElement;
    const observer = new MutationObserver(() => setLight(el.classList.contains("light")));
    observer.observe(el, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return (
    <Sonner
      theme={light ? "light" : "dark"}
      className="toaster group"
      position="bottom-right"
      toastOptions={{
        classNames: {
          toast:
            "group toast group-[.toaster]:bg-card group-[.toaster]:text-card-foreground group-[.toaster]:border-border group-[.toaster]:shadow-lg",
          description: "group-[.toast]:text-muted-foreground",
          actionButton: "group-[.toast]:bg-primary group-[.toast]:text-primary-foreground",
          cancelButton: "group-[.toast]:bg-muted group-[.toast]:text-muted-foreground",
        },
      }}
    />
  );
}
