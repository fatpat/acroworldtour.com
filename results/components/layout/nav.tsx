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
      "fixed bottom-0 z-10 w-full bg-white shadow-lg shadow-awt-dark-900",
      "lg:fixed lg:left-0 lg:h-full lg:w-48 lg:pt-32 lg:shadow-awt-dark-300"
    )}
  >
    {/* This div forces mobile browsers to update the screen size correctly
        when the top address bar collapses */}
    <div className="fixed bottom-0" />
    <ul
      className={classNames(
        "flex w-full justify-evenly pt-2",
        "lg:flex-col lg:items-end lg:gap-4"
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
    <ul
      className={classNames(
        "flex w-full justify-evenly py-2",
        "lg:flex-col lg:items-end lg:gap-4"
      )}
    >
      <li>
        <NavItem link="Teams" active={activeNav === "teams"} />
      </li>
      <li>
        <NavItem link="Judges" active={activeNav === "judges"} />
      </li>
      <li>
        <NavItem link="Tricks" active={activeNav === "tricks"} />
      </li>
    </ul>
  </nav>
);

export default Nav;
