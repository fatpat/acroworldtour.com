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

const NavItem: React.FC<Props> = ({ link, active }) => {
  return (
    <Link
      href={`/${link.toLowerCase()}`}
      className={classNames(
        "flex flex-col items-center justify-center fill-awtgrey-400 text-xs font-semibold text-awtgrey-400",
        "lg:flex-row lg:justify-start lg:rounded-l-xl lg:px-6 lg:py-3 lg:hover:bg-awtgrey-200 lg:hover:shadow-md",
        "hover:fill-sky-600 hover:text-sky-600",
        active && "fill-current text-current bg-awtgrey-100"
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
};

export default NavItem;
