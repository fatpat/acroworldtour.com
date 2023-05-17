import classNames from "classnames";
import Link from "next/link";

import { EventsIcon, HomeIcon, PilotsIcon, ResultsIcon } from "../ui/icons";

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
        "flex flex-col items-center justify-center rounded-l-xl fill-awtgrey-400 text-xs font-semibold text-awtgrey-400",
        "hover:fill-current hover:text-current",
        "lg:flex-row lg:justify-start lg:px-6 lg:py-3 lg:hover:bg-awtgrey-200 lg:hover:shadow-md",
        active && "fill-current text-current"
      )}
    >
      {link === "" && <HomeIcon className={iconClasses} />}
      {link === "Results" && <ResultsIcon className={iconClasses} />}
      {link === "Events" && <EventsIcon className={iconClasses} />}
      {link === "Pilots" && <PilotsIcon className={iconClasses} />}
      <span>{link || "Home"}</span>
    </Link>
  );
};

export default NavItem;
