import Link from "next/link";

import { components } from "@/types";

interface Props {
  competition: components["schemas"]["CompetitionPublicExport"];
}

const Competition: React.FC<Props> = ({ competition }) => {
  const { code: id, name, location, image, start_date, end_date } = competition;
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
      key={id}
      href={`/competitions/${id}`}
      className="max-w-none hover:invert min-w-fit flex-1"
    >
      <article
        style={{ backgroundImage: `url(${image})` }}
        className="max-w-lg rounded-xl bg-neutral-50/95 bg-cover bg-center bg-no-repeat p-4 bg-blend-overlay h-48 flex flex-col justify-between"
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

export default Competition;
