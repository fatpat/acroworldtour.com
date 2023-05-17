import classNames from "classnames";
import Link from "next/link";

import { FacebookIcon, InstagramIcon, YoutubeIcon } from "../ui/icons";

const iconClasses = "hover:bg-awtgrey-200 p-1 rounded-md";

const AWTSocial = () => (
  <>
    <Link
      href={{ pathname: "https://www.instagram.com/acroworldtour/" }}
      target="_blank"
      className={iconClasses + " instagram"}
    >
      <InstagramIcon />
    </Link>
    <Link
      href={{ pathname: "https://www.facebook.com/acroworldtour" }}
      target="_blank"
      className={iconClasses + " facebook"}
    >
      <FacebookIcon />
    </Link>
    <Link
      href={{
        pathname: "https://www.youtube.com/channel/UCZupvXQsTPEdyMSLxBW1RLg",
      }}
      target="_blank"
      className={iconClasses + " youtube"}
    >
      <YoutubeIcon />
    </Link>
  </>
);

export default AWTSocial;
