import { useEffect, useRef, useState } from "react";

/**
 * Returns a ref + a boolean that flips to true the first time the element
 * enters the viewport. Used for fade-up-on-scroll micro-interactions.
 * Defaults to only firing once so animating elements stay in place.
 */
export function useInView<T extends Element = HTMLElement>(options?: IntersectionObserverInit) {
  const ref = useRef<T | null>(null);
  const [inView, setInView] = useState(false);

  useEffect(() => {
    const node = ref.current;
    if (!node || typeof IntersectionObserver === "undefined") {
      // No observer support — show immediately.
      setInView(true);
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setInView(true);
            io.disconnect();
            break;
          }
        }
      },
      { threshold: 0.15, rootMargin: "0px 0px -40px 0px", ...options },
    );
    io.observe(node);
    return () => io.disconnect();
  }, []);

  return { ref, inView } as const;
}
