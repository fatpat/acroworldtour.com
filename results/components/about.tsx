import Link from "next/link";
import { useEffect, useState } from "react";

import {
  ChevronIcon,
  FacebookIcon,
  InstagramIcon,
  YoutubeIcon,
} from "./ui/icons";

const chevronClasses = "w-auto h-3 mt-[1px]";

const About = () => {
  const [hideAbout, setHideAbout] = useState(
    localStorage.getItem("hideAbout") !== null
  );

  useEffect(() => {
    hideAbout
      ? localStorage.setItem("hideAbout", "1")
      : localStorage.removeItem("hideAbout");
  }, [hideAbout]);

  return (
    <article className="flex flex-col items-center flex-1">
      <hgroup
        className={`z-10 mx-auto flex w-1/2 cursor-pointer items-center justify-center ${
          hideAbout ? "-translate-x-1/2 opacity-50" : ""
        }`}
        onClick={() => setHideAbout(!hideAbout)}
      >
        <h2 className={`mx-2 ${hideAbout ? "text-sm" : ""}`}>About</h2>
        <ChevronIcon
          className={`${chevronClasses} ${hideAbout ? "-rotate-90" : ""} `}
        />
      </hgroup>
      <div
        className={`flex h-56 flex-col items-center justify-evenly rounded-xl bg-awtgrey-100 px-4 font-medium ${
          hideAbout ? "-mt-64 opacity-0" : ""
        }`}
      >
        <p>
          This is the official web application of the Acro World Tour. In this
          app you can find everything you need to know about competition results
          and your favourite pilots.
        </p>
        <p>
          To stay up to date about the Acro World Tour, follow our social media
          down below.
        </p>
      </div>
      <div
        className={`mx-auto flex w-1/2 items-center justify-evenly ${
          hideAbout ? " mb-4 translate-x-1/2" : "my-4"
        }`}
      >
        <Link
          href={{ pathname: "https://www.instagram.com/acroworldtour/" }}
          target="_blank"
        >
          <InstagramIcon />
        </Link>
        <Link
          href={{ pathname: "https://www.facebook.com/acroworldtour" }}
          target="_blank"
        >
          <FacebookIcon />
        </Link>
        <Link
          href={{
            pathname:
              "https://www.youtube.com/channel/UCZupvXQsTPEdyMSLxBW1RLg",
          }}
          target="_blank"
        >
          <YoutubeIcon />
        </Link>
      </div>
    </article>
  );
};

export default About;
