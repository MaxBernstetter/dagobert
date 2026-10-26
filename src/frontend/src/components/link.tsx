import type { MouseEvent, ReactNode } from "react";
import { Link as RouterLink } from "react-router-dom";

type Props = {
  href: string;
  children?: ReactNode;
  className?: string;
  onClick?: (event: MouseEvent<HTMLAnchorElement>) => void;
  title?: string;
  "aria-label"?: string;
};

export default function Link({ href, children, ...rest }: Props) {
  return (
    <RouterLink to={href} {...rest}>
      {children}
    </RouterLink>
  );
}
