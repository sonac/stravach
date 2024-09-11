import TelegramLoginButton, { TelegramUser } from "./TelegramAuth";
import "./App.css";
import {Button} from "./ui/button.tsx";

function onTelegramAuth(user: any) {
  const payload = {
    user: {
      id: user.id,
      first_name: user.first_name,
      last_name: user.last_name,
      username: user.username,
    },
  };

  fetch("/tg-auth", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
    credentials: "include",
  })
    .then((response) => {
      if (response.ok) {
        return response.text();
      } else {
        throw new Error("Failed to authenticate");
      }
    })
    .then((data) => {
      console.log("Authentication successful:", data);
      window.location.href = "/user/" + user.id;
    })
    .catch((error) => {
      console.error("Error:", error);
      alert("Authentication failed. Please try again.");
    });
}

function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <section className="bg-blue-600 text-white py-16 text-center">
        <h1 className="text-4xl font-bold mb-4">Make Every Workout Memorable</h1>
        <p className="text-lg mb-6">Automatically generate creative names for your Strava activities</p>
        <Button className="px-28 py-14 text-lg font-bold rounded-lg shadow-lg"  onClick={() => window.open("https://t.me/strava_snitch_bot", "_blank")}>
          Try the Bot
        </Button>
        <TelegramLoginButton
          botName="strava_snitch_bot"
          dataOnauth={(user: TelegramUser) => onTelegramAuth(user)}
        />
      </section>
      <section className="py-16 bg-white">
        <div className="container mx-auto text-center">
          <h2 className="text-3xl font-semibold mb-8">How It Works</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div>
              <h3 className="text-xl font-bold mb-2">Step 1</h3>
              <p>Join the bot and start a conversation.</p>
            </div>
            <div>
              <h3 className="text-xl font-bold mb-2">Step 2</h3>
              <p>Do some workout that will be uploaded to strava.</p>
            </div>
            <div>
              <h3 className="text-xl font-bold mb-2">Step 3</h3>
              <p>Get some creative names for your activity instantly!</p>
            </div>
          </div>
        </div>
      </section>
      <section className="py-16 bg-gray-100">
        <div className="container mx-auto text-center">
          <h2 className="text-3xl font-semibold mb-8">
            Sometimes I go for a run, just because I'm bored. <br/>
            And I go there not for the boring "Evening Run" names in my Strava!
          </h2>
          <div className="flex flex-col md:flex-row justify-center items-center gap-8">
            {/* Boy Runner */}
            <div>
              <img
                src="/anime_runner_1.webp"
                alt="Boy runner"
                className="w-full max-w-xs mx-auto"
              />
            </div>
            {/* Girl Runner */}
            <div>
              <img
                src="/anime_runner_2.webp"
                alt="Girl runner"
                className="w-full max-w-xs mx-auto"
              />
            </div>
          </div>
        </div>
      </section>
      <section className="py-16 bg-gray-100">
        <div className="container mx-auto text-center">
          <h2 className="text-3xl font-semibold mb-8">Features</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div>
              <h3 className="text-xl font-bold mb-2">Creative Suggestions</h3>
              <p>Get unique and fun names for your activities.</p>
            </div>
            <div>
              <h3 className="text-xl font-bold mb-2">Customizable Options</h3>
              <p>Adjust names based on your workout type.</p>
            </div>
            <div>
              <h3 className="text-xl font-bold mb-2">Sync to Strava</h3>
              <p>Easily connect and sync your name directly to Strava.</p>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

export default App;
