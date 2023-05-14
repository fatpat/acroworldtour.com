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
      "fixed bottom-0 z-10 flex h-20 w-full justify-evenly bg-white shadow-md shadow-current",
      "lg:left-0 lg:top-0 lg:mt-24 lg:h-full lg:w-52 lg:flex-col lg:justify-start lg:bg-transparent lg:bg-white lg:pl-4 lg:pt-8 lg:shadow-lg"
    )}
  >
    <NavItem link="" active={pageTitle.includes("Home")} />
    <NavItem link="Results" active={pageTitle.includes("Results")} />
    <NavItem link="Events" active={pageTitle.includes("Events")} />
    <NavItem link="Pilots" active={pageTitle.includes("Pilots")} />
  </nav>
);

export default Nav;
