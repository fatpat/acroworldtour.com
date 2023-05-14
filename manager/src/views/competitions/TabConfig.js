// ** React Imports
import { useState, useRef } from 'react'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import TextField from '@mui/material/TextField'
import InputLabel from '@mui/material/InputLabel'
import FormControl from '@mui/material/FormControl'
import Select from '@mui/material/Select'
import MenuItem from '@mui/material/MenuItem'
import Typography from '@mui/material/Typography'

// ** local
import Editable from 'src/components/Editable'

const TabConfig = ({ config,  update, type}) => {
  // ** State
  const [value, setValue] = useState(config)

  // ** Refs
  const warningRef = useRef()
  const malusRepetitionRef = useRef()
  const warningsToDSQRef = useRef()
  const judgeWeightSeniorRef = useRef()
  const judgeWeightCertifiedRef = useRef()
  const judgeWeightTraineeRef = useRef()
  const markTechnicalRef = useRef()
  const markChoreographyRef = useRef()
  const markLandingRef = useRef()
  const markSynchroRef = useRef()
  const maxBonusTwistRef = useRef()
  const maxBonusReverseRef = useRef()
  const maxBonusFlipRef = useRef()

  return (
    <CardContent>
      <Grid container spacing={7}>
        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.warning}
              title="Warning"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={warningRef}
            >
              <TextField
                fullWidth name="warning" label='Warning' placeholder='Warning' defaultValue={config.warning} inputProps={ {ref:warningRef} }
                onChange={(e) => {
                  value.warning = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.malus_repetition}
              title="Malus Repetition"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={malusRepetitionRef}
            >
              <TextField
                fullWidth name="malus_repetition" label='Malus Repetition' placeholder='Malus Repetition' defaultValue={config.malus_repetition} inputProps={ {ref:malusRepetitionRef} }
                onChange={(e) => {
                  value.malus_repetition = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.warnings_to_dsq}
              title="Warnings to DSQ"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={warningsToDSQRef}
            >
              <TextField
                fullWidth name="warnings_to_dsq" label='Warnings to DSQ' placeholder='Warnings to DSQ' defaultValue={config.warnings_to_dsq} inputProps={ {ref:warningsToDSQRef} }
                onChange={(e) => {
                  value.warnings_to_dsq = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.judge_weights.senior}
              title="Senior Judge Weight"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={judgeWeightSeniorRef}
            >
              <TextField
                fullWidth name="judge_weight_senior" label='Senior Judge Weight' placeholder='Senior Judge Weight' defaultValue={config.judge_weights.senior} inputProps={ {ref:judgeWeightSeniorRef} }
                onChange={(e) => {
                  value.judge_weights.senior = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.judge_weights.certified}
              title="Certified Judge Weight"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={judgeWeightCertifiedRef}
            >
              <TextField
                fullWidth name="judge_weight_certified" label='Certified Judge Weight' placeholder='Certified Judge Weight' defaultValue={config.judge_weights.certified} inputProps={ {ref:judgeWeightCertifiedRef} }
                onChange={(e) => {
                  value.judge_weights.certified = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.judge_weights.trainee}
              title="Trainee Judge Weight"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={judgeWeightTraineeRef}
            >
              <TextField
                fullWidth name="judge_weight_trainee" label='Trainee Judge Weight' placeholder='Trainee Judge Weight' defaultValue={config.judge_weights.trainee} inputProps={ {ref:judgeWeightTraineeRef} }
                onChange={(e) => {
                  value.judge_weights.trainee = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.max_bonus_per_run.twist}
              title="Max Twists per run"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={maxBonusTwistRef}
            >
              <TextField
                fullWidth name="max_twist" label='Max Twists per run' placeholder='Max Twists per run' defaultValue={config.max_bonus_per_run.twist} inputProps={ {ref:maxBonusTwistRef} }
                onChange={(e) => {
                  value.max_bonus_per_run.twist = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.max_bonus_per_run.reverse}
              title="Max Reverses per run"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={maxBonusReverseRef}
            >
              <TextField
                fullWidth name="max_reverse" label='Max Reverses per run' placeholder='Max Reverses per run' defaultValue={config.max_bonus_per_run.reverse} inputProps={ {ref:maxBonusReverseRef} }
                onChange={(e) => {
                  value.max_bonus_per_run.reverse = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.max_bonus_per_run.flip}
              title="Max Flips per run"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={maxBonusFlipRef}
            >
              <TextField
                fullWidth name="max_flip" label='Max Flips per run' placeholder='Max Flips per run' defaultValue={config.max_bonus_per_run.flip} inputProps={ {ref:maxBonusFlipRef} }
                onChange={(e) => {
                  value.max_bonus_per_run.flip = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.mark_percentages[type].technical}
              title="Mark Percentage: Technical"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={markTechnicalRef}
            >
              <TextField
                fullWidth name="mark_percentages_technical" label='Mark Percentage: Technical' placeholder='Technical' defaultValue={config.mark_percentages[type].technical} inputProps={ {ref:markTechnicalRef} }
                onChange={(e) => {
                  value.mark_percentages[type].technical = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.mark_percentages[type].choreography}
              title="Mark Percentage: Choreography"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={markChoreographyRef}
            >
              <TextField
                fullWidth name="mark_percentages_choreography" label='Mark Percentage: Choreography' placeholder='Choreography' defaultValue={config.mark_percentages[type].choreography} inputProps={ {ref:markChoreographyRef} }
                onChange={(e) => {
                  value.mark_percentages[type].choreography = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.mark_percentages[type].landing}
              title="Mark Percentage: Landing"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={markLandingRef}
            >
              <TextField
                fullWidth name="mark_percentages_landing" label='Mark Percentage: Landing' placeholder='Landing' defaultValue={config.mark_percentages[type].landing} inputProps={ {ref:markLandingRef} }
                onChange={(e) => {
                  value.mark_percentages[type].landing = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>

{ type == "synchro" && 
        <Grid item xs={4} sm={4} container>
          <Typography>
            <Editable
              text={config.mark_percentages[type].synchro}
              title="Mark Percentage: Synchro"
              onChange={(e) => update(value)}
              onCancel={(e) => {
                setValue(config)
              }}
              childRef={markSynchroRef}
            >
              <TextField
                fullWidth name="mark_percentages_synchro" label='Mark Percentage: Synchro' placeholder='Synchro' defaultValue={config.mark_percentages[type].synchro} inputProps={ {ref:markSynchroRef} }
                onChange={(e) => {
                  value.mark_percentages[type].synchro = e.target.value
                  setValue(value)
                }}
              />
            </Editable>
          </Typography>
        </Grid>
}

      </Grid>
    </CardContent>
  )
}

export default TabConfig
