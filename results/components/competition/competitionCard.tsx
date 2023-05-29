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
      className="w-full max-w-lg hover:-translate-y-2 hover:invert"
    >
      <article
        style={{ backgroundImage: `url(${image})` }}
        className="card flex h-48 flex-col justify-between p-4"
      >
        <hgroup>
          <h3 className="text-left">{name}</h3>
          <h4 className="text-left">{location}</h4>
        </hgroup>
        <div className="flex justify-between">
          <small>{`${startDay} ${startMonth !== endMonth ? startMonth : ""} ${
            startYear !== endYear ? startYear : ""
          } to ${endDay} ${endMonth} ${endYear}`}</small>
          <small>{`${numberOfPilots} pilots`}</small>
        </div>
      </article>
    </Link>
  );
};

export default CompetitionCard;
