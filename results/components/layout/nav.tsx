import classNames from "classnames";
import { Montserrat } from "next/font/google";

import NavItem from "./navItem";

const montserrat = Montserrat({
  subsets: ["latin"],
});

interface Props {
  activeNav: string;
}

const Nav = ({ activeNav }: Props) => (
  <nav
    className={classNames(
      montserrat.className,
      "sticky bottom-0 z-10 w-full bg-white shadow-md shadow-current",
      "lg:fixed lg:left-0 lg:h-full lg:w-52 lg:pl-4 lg:pt-32 lg:shadow-lg"
    )}
  >
    <ul
      className={classNames(
        "flex h-20 w-full items-center justify-evenly",
        "lg:h-auto lg:flex-col lg:items-end lg:justify-start lg:gap-4"
      )}
    >
      <li>
        <NavItem link="" active={activeNav === "home"} />
      </li>
      <li>
        <NavItem link="Seasons" active={activeNav === "seasons"} />
      </li>
      <li>
        <NavItem link="Competitions" active={activeNav === "competitions"} />
      </li>
      <li>
        <NavItem link="Pilots" active={activeNav === "pilots"} />
      </li>
    </ul>
  </nav>
);

export default Nav;
