import classNames from "classnames";
import { type NextPage } from "next";
import { Montserrat } from "next/font/google";
import Image from "next/image";
import Link from "next/link";

import lines from "/img/lines.svg";
import awtLogo from "/img/logo.svg";

const montserrat = Montserrat({
  subsets: ["latin"],
});

interface Props {
  headerTitle?: string;
  headerSubtitle?: string;
}

const Header: NextPage<Props> = ({ headerTitle, headerSubtitle }) => (
  <header
    className={classNames(
      montserrat.className,
      "z-20 flex h-24 w-full items-center justify-between bg-awtgrey-900 bg-contain bg-right bg-no-repeat px-6",
      "lg:fixed lg:top-0"
    )}
    style={{
      backgroundImage: `url(${lines.src})`,
    }}
  >
    <Link href="/">
      <Image
        src={awtLogo}
        alt="Acro World Tour logo"
        width="0"
        height="0"
        className="h-11 w-auto"
      />
    </Link>
    <hgroup className="text-right">
      <h2 className="text-sm font-medium text-awtgrey-400">{headerSubtitle}</h2>
      <h1 className="text-white">{headerTitle}</h1>
    </hgroup>
  </header>
);

export default Header;
