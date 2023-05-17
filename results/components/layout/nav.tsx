import classNames from "classnames";
import { Montserrat } from "next/font/google";

import NavItem from "./navItem";

const montserrat = Montserrat({
  subsets: ["latin"],
});

interface Props {
  pageTitle: string;
}

const Nav = ({ pageTitle }: Props) => (
  <nav
    className={classNames(
      montserrat.className,
      "sticky bottom-0 z-10 w-full bg-white shadow-md shadow-current",
      "lg:fixed lg:h-full lg:left-0 lg:w-52 lg:pl-4 lg:shadow-lg lg:pt-32"
    )}
  >
    <ul
      className={classNames(
        "flex h-20 w-full items-center justify-evenly",
        "lg:flex-col lg:items-end lg:justify-start lg:gap-4 lg:h-auto"
      )}
    >
      <li>
        <NavItem link="" active={pageTitle?.includes("Home")} />
      </li>
      <li>
        <NavItem link="Results" active={pageTitle?.includes("Results")} />
      </li>
      <li>
        <NavItem link="Events" active={pageTitle?.includes("Events")} />
      </li>
      <li>
        <NavItem link="Pilots" active={pageTitle?.includes("Pilots")} />
      </li>
    </ul>
  </nav>
);

export default Nav;
