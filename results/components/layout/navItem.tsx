import classNames from "classnames";
import Link from "next/link";

import {
  CompetitionsIcon,
  HomeIcon,
  JudgesIcon,
  PilotsIcon,
  SeasonsIcon,
  TeamsIcon,
  TricksIcon,
} from "../ui/icons";

interface Props {
  link: string;
  active: boolean;
}

const iconClasses = classNames("fill-inherit h-6 w-auto", "lg:pr-3");

const NavItem = ({ link, active }: Props) => (
  <Link
    href={`/${link.toLowerCase()}`}
    className={classNames(
      "flex flex-col items-center justify-center rounded-xl p-2 text-xs font-semibold text-awt-dark-600",
      "lg:flex-row lg:justify-start lg:rounded-r-none lg:py-3 lg:pl-6 lg:text-base",
      !active &&
        "hover:bg-awt-accent-600 hover:fill-white hover:text-white hover:shadow-md",
      active
        ? "bg-awt-dark-900 fill-white text-white shadow-md"
        : "fill-awt-dark-600"
    )}
  >
    {link === "" && <HomeIcon className={iconClasses} />}
    {link === "Seasons" && <SeasonsIcon className={iconClasses} />}
    {link === "Competitions" && <CompetitionsIcon className={iconClasses} />}
    {link === "Pilots" && <PilotsIcon className={iconClasses} />}
    {link === "Teams" && <TeamsIcon className={iconClasses} />}
    {link === "Judges" && <JudgesIcon className={iconClasses} />}
    {link === "Tricks" && <TricksIcon className={iconClasses} />}
    <span>{link || "Home"}</span>
  </Link>
);

export default NavItem;
