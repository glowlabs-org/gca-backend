# watttime-scripts

Scripts to fetch data from external API sources. 

## WattTime

Details of the WattTime API: [WattTime V3 Docs](https://docs.watttime.org/).

The scripts query the [Marginal CO2](https://watttime.org/data-science/data-signals/marginal-co2/) rate.

WattTime's V3 API returns [JSON:API](https://jsonapi.org/) format.

## NASA Power

Details of NASA Power API: [NASA Power API](https://power.larc.nasa.gov/docs/services/api/).

The scripts use the [Renewable Energy (RE)](https://power.larc.nasa.gov/docs/methodology/communities/) community,
and query using the [Solar Irradiance (ALLSKY_SFC_SW_DWN)](https://power.larc.nasa.gov/docs/gallery/solar-irradiance/) parameter.