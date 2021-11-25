# Learnt API Config

We've implemented a Config struct that embeds viper. We unmarshall the yaml file to this struct to make it easier to access without wild-guessing
using GetConfig().GetString('variable'), and then doing tedious type-juggling.

We've embedded viper on this config so it's still possible to do GetConfig().GetString('variable'), etc (especially if you've just wanted 
to quickly try out a new API).

## Usage

```
	c := GetConfig()
	c.App.Listen //  "0.0.0.0:5050"
	c.App.SearchMiles // 50
```


