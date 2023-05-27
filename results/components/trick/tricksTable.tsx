import { Col, Row, Table } from "@nextui-org/react";
import Link from "next/link";

import { components } from "@/types";

import { YouTubeIcon } from "../ui/icons";

interface Props {
  tricks: components["schemas"]["Trick"][];
}

const TricksTable = ({ tricks }: Props) => {
  return (
    <Table aria-label="Tricks table" className="w-4">
      <Table.Header>
        <Table.Column align="center">Trick</Table.Column>
        <Table.Column align="center">Acronym</Table.Column>
        <Table.Column align="center">Properties</Table.Column>
        <Table.Column align="center">Bonuses</Table.Column>
        <Table.Column align="center">Technical Coefficient</Table.Column>
      </Table.Header>
      <Table.Body>
        {tricks
          .sort((a, b) => a.technical_coefficient - b.technical_coefficient)
          .map((trick) => (
            <Table.Row key={trick._id}>
              <Table.Cell>{trick.name}{trick.sample_video && <Link href={trick.sample_video}><YouTubeIcon /></Link>}</Table.Cell>
              <Table.Cell>{trick.acronym}</Table.Cell>
              <Table.Cell>
                {trick.directions.includes("left") && (
                  <label htmlFor={`directions-${trick._id}`} title="Right">
                    ⬅️
                  </label>
                )}
                {trick.directions.includes("right") && (
                  <label htmlFor={`directions-${trick._id}`} title="Left">
                    ➡️
                  </label>
                )}
                {(trick.directions.includes("opposite") ||
                  trick.directions.includes("mirror")) && (
                  <label
                    htmlFor={`directions-${trick._id}`}
                    title="Opposite/Mirror"
                  >
                    ↔️
                  </label>
                )}
                {trick.repeatable && (
                  <label
                    htmlFor={`repeatable-${trick._id}`}
                    title="this trick can be repeated"
                  >
                    🔁
                  </label>
                )}
                {(trick.first_maneuver || 0) > 0 && (
                  <label
                    htmlFor={`first-maneuver-${trick._id}`}
                    title="this trick must be performed as a first trick only"
                  >
                    ①
                  </label>
                )}
                {(trick.no_last_maneuver || 0) > 0 && (
                  <label
                    htmlFor={`no-last-maneuver-${trick._id}`}
                    title={`this trick cannot be performed as a last ${trick.no_last_maneuver} maneuver(s)`}
                  >
                    ⨂
                  </label>
                )}
              </Table.Cell>
              <Table.Cell>
                {trick.bonuses
                  .sort((a, b) => a.bonus - b.bonus)
                  .map((bonus) => (
                    <Row key={`${trick._id}-${bonus.name}`}>
                      <Col>{bonus.name}</Col>
                      <Col>{bonus.bonus}%</Col>
                    </Row>
                  ))}
              </Table.Cell>
              <Table.Cell>{trick.technical_coefficient.toFixed(2)}</Table.Cell>
            </Table.Row>
          ))}
      </Table.Body>
    </Table>
  );
};

export default TricksTable;
