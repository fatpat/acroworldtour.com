import classNames from "classnames";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  competition: components["schemas"]["CompetitionPublicExport"];
}

const CompetitionCard = ({ competition }: Props) => {
  const {
    code,
    name,
    location,
    image,
    start_date,
    end_date,
    number_of_pilots: numberOfPilots,
  } = competition;
  const startDate = new Date(start_date);
  const endDate = new Date(end_date);
  const startDay = startDate.getDate();
  const startMonth = startDate.toLocaleString("default", {
    month: "long",
  });
  const startYear = startDate.getFullYear();
  const endDay = endDate.getDate();
  const endMonth = endDate.toLocaleString("default", {
    month: "long",
  });
  const endYear = endDate.getFullYear();
  return (
    <Link
      key={code}
      href={`/competitions/${code}`}
      className={classNames("max-w-lg flex-grow")}
    >
      <article
        style={{ backgroundImage: `url(${image})` }}
        className={classNames(
          "flex h-48 flex-col justify-between rounded-xl text-white",
          "bg-black/60 bg-cover bg-center bg-no-repeat p-4 bg-blend-multiply",
          "shadow shadow-awt-dark-400",
          "hover:-translate-y-2 hover:bg-white/90 hover:text-current hover:bg-blend-screen",
          "hover:shadow-md",
        )}
      >
        <hgroup>
          <h3 className="text-left">{name}</h3>
          <h4 className="text-left">{location}</h4>
        </hgroup>
        <div className="flex justify-between">
          <p>{`${startDay} ${startMonth !== endMonth ? startMonth : ""} ${
            startYear !== endYear ? startYear : ""
          } to ${endDay} ${endMonth} ${endYear}`}</p>
          <p>{`${numberOfPilots} pilots`}</p>
        </div>
      </article>
    </Link>
  );
};

export default CompetitionCard;
