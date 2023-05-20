import classNames from "classnames";
import { Montserrat } from "next/font/google";
import Image from "next/image";
import Link from "next/link";
import { useEffect, useState } from "react";

import lines from "/img/lines.svg";
import awtLogo from "/img/logo.svg";

const montserrat = Montserrat({
  subsets: ["latin"],
});

interface Props {
  headerTitle: string;
  headerSubtitle: string;
}

const Header = (
  { headerTitle, headerSubtitle }: Props
) => {
  return (
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
        <h2 className="text-base font-medium text-awtgrey-400 m-0">
          {headerSubtitle}
        </h2>
        <h1 className="text-white m-0">
          {headerTitle}
        </h1>
      </hgroup>
    </header>
  );
};

export default Header;
