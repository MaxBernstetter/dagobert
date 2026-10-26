import { useLocation } from "react-router-dom";

export function usePathname(): string {
  return useLocation().pathname;
}
