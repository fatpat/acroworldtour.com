import Link from "next/link";

import { FacebookIcon, InstagramIcon, TikTokIcon, TwitterIcon, WebsiteIcon,WikipediaIcon, YouTubeIcon} from "./icons";

interface Props {
  className?: string;
  link: string;
  media:
    | "facebook"
    | "instagram"
    | "tiktok"
    | "twitter"
    | "wikipedia"
    | "youtube"
    | string;
}

const SocialLink = ({ className, link, media }: Props) => {
  const mediaName = media.toLowerCase();
  return (
    <Link
      href={{ pathname: link }}
      title={media}
      target="_blank"
      className={className}
      rel="noopener noreferrer"
    >
      {(mediaName === "facebook" && <FacebookIcon />) ||
        (mediaName === "instagram" && <InstagramIcon />) ||
        (mediaName === "tiktok" && <TikTokIcon />) ||
        (mediaName === "twitter" && <TwitterIcon />) ||
        (mediaName === "wikipedia" && <WikipediaIcon />) ||
        (mediaName === "youtube" && <YouTubeIcon />) || <WebsiteIcon />}
    </Link>
  );
};

export default SocialLink;
