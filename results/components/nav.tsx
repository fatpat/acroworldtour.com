import { Montserrat } from "next/font/google";

import NavItem from "./navItem";

const montserrat = Montserrat({
  subsets: ["latin"],
});

const Nav = () => (
  <nav
    className={`${montserrat.className}
      fixed bottom-0 z-10 flex h-20 w-full justify-evenly bg-white shadow-md shadow-current
      lg:top-0 lg:left-0 lg:mt-24 lg:w-52 lg:flex-col lg:justify-start lg:pt-8 lg:bg-transparent lg:pl-4 lg:shadow-lg lg:bg-white lg:h-full`}
  >
    <NavItem link="" />
    <NavItem link="Results" />
    <NavItem link="Events" />
    <NavItem link="Pilots" />
  </nav>
);

export default Nav;
