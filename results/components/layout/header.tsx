import classNames from "classnames";
import Image from "next/image";
import Link from "next/link";

import lines from "/img/lines.svg";
import awtLogo from "/img/logo.svg";

interface Props {
  fontClass: string;
  headerTitle: string;
  headerSubtitle: string;
}

const Header = ({ fontClass, headerTitle, headerSubtitle }: Props) => {
  return (
    <header
      className={classNames(
        fontClass,
        "z-20 flex h-24 w-full items-center justify-between bg-awt-dark-950 bg-contain bg-right bg-no-repeat px-6 text-white",
        "lg:fixed lg:top-0",
      )}
      style={{
        backgroundImage: `url(${lines.src})`,
      }}
    >
      <Link href="/" title="Navigate Home">
        <Image
          src={awtLogo}
          alt="Acro World Tour logo"
          width="0"
          height="0"
          className="h-11 w-auto"
        />
      </Link>
      <hgroup>
        <h2 className="text-right text-base capitalize opacity-70 lg:text-lg">
          {headerSubtitle}
        </h2>
        <h1 className="text-right capitalize">{headerTitle}</h1>
      </hgroup>
    </header>
  );
};

export default Header;
