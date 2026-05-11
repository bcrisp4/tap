export type MarkOnScrollOpts = {
  onMark: () => void;
  delayMs?: number;
};

export function createMarkOnScroll(opts: MarkOnScrollOpts) {
  const delay = opts.delayMs ?? 1500;
  return (el: Element) => {
    let fired = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const observer = new IntersectionObserver((records) => {
      for (const r of records) {
        const aboveViewport = !r.isIntersecting && r.boundingClientRect.top < 0;
        if (aboveViewport && !fired && timer === null) {
          timer = setTimeout(() => {
            timer = null;
            if (fired) return;
            fired = true;
            opts.onMark();
          }, delay);
        } else if (!aboveViewport && timer !== null) {
          clearTimeout(timer);
          timer = null;
        }
      }
    }, { threshold: 0 });

    observer.observe(el);
    return () => {
      if (timer !== null) clearTimeout(timer);
      observer.disconnect();
    };
  };
}
