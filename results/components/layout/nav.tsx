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
      "fixed bottom-0 z-10 h-20 w-full bg-white shadow-md shadow-current",
      "lg:left-0 lg:top-0 lg:mt-24 lg:h-full lg:w-52 lg:bg-transparent lg:bg-white lg:pl-4 lg:pt-8 lg:shadow-lg"
    )}
  >
    <ul
      className={classNames(
        "flex h-full w-full items-center justify-evenly",
        "lg:flex-col lg:items-end lg:justify-start lg:gap-4"
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
