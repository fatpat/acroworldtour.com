import Link from "next/link";

import { components } from "@/types";

interface Props {
  competition: components["schemas"]["CompetitionPublicExport"];
}

const CompetitionCard = ({ competition }: Props) => {
  const { code, name, location, image, start_date, end_date } = competition;
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
      className="w-full max-w-sm hover:invert"
    >
      <article
        style={{ backgroundImage: `url(${image})` }}
        className="flex h-48 flex-col justify-between rounded-xl bg-neutral-50/95 bg-cover bg-center bg-no-repeat p-4 bg-blend-overlay"
      >
        <hgroup className="mb-12">
          <h3>{name}</h3>
          <h4>{location}</h4>
        </hgroup>
        <small>{`${startDay} ${startMonth !== endMonth ? startMonth : ""} ${
          startYear !== endYear ? startYear : ""
        } to ${endDay} ${endMonth} ${endYear}`}</small>
      </article>
    </Link>
  );
};

export default CompetitionCard;
