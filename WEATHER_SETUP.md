# Weather API Setup Guide

## OpenWeatherMap Free API Integration

This project now uses OpenWeatherMap's free API to provide real weather forecasts.

### Getting Your Free API Key

1. **Sign up for OpenWeatherMap**
   - Go to: https://openweathermap.org/api
   - Click "Sign Up" (top right)
   - Create a free account

2. **Get Your API Key**
   - After signing up, go to: https://home.openweathermap.org/api_keys
   - Your default API key will be shown (or create a new one)
   - Copy the API key

3. **Set the Environment Variable**

   **On macOS/Linux:**
   ```bash
   export OPENWEATHER_API_KEY="your_api_key_here"
   ```

   **On Windows (PowerShell):**
   ```powershell
   $env:OPENWEATHER_API_KEY="your_api_key_here"
   ```

   **Or add to your shell profile (.zshrc, .bashrc, etc.):**
   ```bash
   echo 'export OPENWEATHER_API_KEY="your_api_key_here"' >> ~/.zshrc
   source ~/.zshrc
   ```

4. **Start the Application**
   ```bash
   go run src/backend/*.go
   ```

### API Key Activation

⚠️ **Important:** New API keys may take 10 minutes to 2 hours to activate after creation.

If you get an error immediately after creating your key, wait 10-15 minutes and try again.

### What the Free Tier Includes

- ✅ 1,000 API calls per day
- ✅ Current weather data
- ✅ Temperature in Celsius
- ✅ Weather conditions (sunny, cloudy, rainy, etc.)
- ✅ Weather for any city worldwide

### How It Works

1. User enters a city name on the homepage
2. Backend makes API call to OpenWeatherMap
3. Receives real-time weather data (temperature, conditions)
4. Displays formatted weather information

### Fallback Behavior

If the API key is not set or the API call fails:
- The app shows an error message
- Suggests checking the city name
- No crash - graceful degradation

### Testing

After setting up your API key, test it:

```bash
# Test with curl
curl "http://localhost:8081/api/weather?city=Copenhagen"

# Or use the web interface
# Go to http://localhost:8081 and use the weather form
```

### Troubleshooting

**"OPENWEATHER_API_KEY environment variable not set"**
- Make sure you exported the environment variable
- Restart your terminal/IDE after setting it

**"weather API returned status 401"**
- Your API key is invalid or not yet activated
- Wait 10-15 minutes after creating the key
- Check you copied the entire key correctly

**"weather API returned status 404"**
- City name not found
- Try different spelling or use country code: "London,UK"

### Example Cities to Try

- Copenhagen
- London
- New York
- Tokyo
- Paris
- Berlin
- Sydney

### API Documentation

Full OpenWeatherMap API docs: https://openweathermap.org/current
