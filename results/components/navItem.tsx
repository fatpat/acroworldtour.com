import Link from "next/link";

import { EventsIcon, HomeIcon, PilotsIcon, ResultsIcon } from "./ui/icons";

interface Props {
  link: string;
}

const iconClassName = "fill-inherit h-6 w-auto lg:pr-3";

const NavItem: React.FC<Props> = ({ link }) => (
  <Link
    href={`/${link.toLowerCase()}`}
    className="flex flex-col items-center justify-center rounded-l-xl fill-awtgrey-400 text-xs font-semibold text-awtgrey-400 focus:fill-current focus:text-current lg:flex-row lg:justify-start lg:px-6 lg:py-3 lg:hover:bg-awtgrey-200 lg:hover:fill-current lg:hover:text-current lg:focus:bg-awtgrey-200"
  >
    {link === "" && <HomeIcon className={iconClassName} />}
    {link === "Results" && <ResultsIcon className={iconClassName} />}
    {link === "Events" && <EventsIcon className={iconClassName} />}
    {link === "Pilots" && <PilotsIcon className={iconClassName} />}
    <span>{link || "Home"}</span>
  </Link>
);

export default NavItem;
