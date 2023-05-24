import classNames from "classnames";

import NavItem from "./navItem";

interface Props {
  activeNav: string;
  fontClass: string;
}

const Nav = ({ activeNav, fontClass }: Props) => (
  <nav
    className={classNames(
      fontClass,
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
      <li>
        <NavItem link="Teams" active={activeNav === "Teams"} />
      </li>
      <li>
        <NavItem link="Judges" active={activeNav === "Judges"} />
      </li>
    </ul>
  </nav>
);

export default Nav;
