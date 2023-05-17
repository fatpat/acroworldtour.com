import classNames from "classnames";
import { ReactNode, useEffect, useState } from "react";

import { ChevronIcon } from "../ui/icons";
import AWTSocial from "./awtSocial";

interface Props {
  content: ReactNode;
  social: ReactNode;
}

const iconClasses = "hover:bg-awtgrey-200 p-1 rounded-md";

const About = ({ content, social }: Props) => {
  const [hideAbout, setHideAbout] = useState(
    localStorage.getItem("hideAbout") !== null
  );

  useEffect(() => {
    hideAbout
      ? localStorage.setItem("hideAbout", "1")
      : localStorage.removeItem("hideAbout");
  }, [hideAbout]);

  return (
    <article className="flex flex-col items-center">
      <header
        className={classNames(
          "z-10 mb-1 flex w-1/2 cursor-pointer items-center justify-center",
          hideAbout && "-translate-x-3/4 opacity-50"
        )}
        onClick={() => setHideAbout(!hideAbout)}
      >
        <h2 className={classNames("m-2", hideAbout && "text-sm")}>About</h2>
        <ChevronIcon
          className={classNames(
            "mt-[1px] h-3 w-auto",
            hideAbout && "-rotate-90"
          )}
        />
      </header>
      <div
        className={classNames(
          "flex h-56 flex-col items-center justify-evenly rounded-xl bg-awtgrey-100 px-4 font-medium shadow",
          hideAbout && "-mt-64 opacity-0"
        )}
      >
        {content}
      </div>
      <footer
        className={classNames(
          "flex w-1/2 max-w-[150px] items-center justify-between",
          "lg: max-w-xs",
          hideAbout ? "translate-x-1/2" : "mt-2"
        )}
      >
        {social}
      </footer>
    </article>
  );
};

export default About;
