import classNames from "classnames";

// import countries, { alpha3ToAlpha2 } from "i18n-iso-countries";
import { components } from "@/types";

// countries.registerLocale(require("i18n-iso-countries/langs/en.json"));

interface Props {
  team: components["schemas"]["TeamExport"];
}

const TeamDetails = ({ team }: Props) => {
  // const { name, pilots } = team;

  return (
    <>
      <h2 className="mb-4">{team.name}</h2>
      <section className={classNames("flex w-full flex-col")}></section>
    </>
  );
};

export default TeamDetails;
