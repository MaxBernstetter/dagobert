import { useLocation } from "react-router-dom";

export function usePathname(): string {
  return useLocation().pathname;
}

export function useRouter() {
  return {
    push: (href: string) => {
      window.location.assign(href);
    },
    refresh: () => {
      window.location.reload();
    },
  };
}
