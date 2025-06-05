import TelegramLoginButton, { TelegramUser } from "./TelegramAuth";
import "./App.css";
import { Button } from "./ui/button.tsx";
import { Routes, Route, Link, useNavigate } from "react-router-dom";
import ActivitiesPage from "../pages/ActivitiesPage";
import UserProfilePage from "../pages/UserProfilePage";

function onTelegramAuth(
  user: TelegramUser,
  navigate: ReturnType<typeof useNavigate>,
) {
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
      navigate(`/user/${user.id}`);
    })
    .catch((error) => {
      console.error("Error:", error);
      alert("Authentication failed. Please try again.");
    });
}

const HomePage = () => {
  const navigate = useNavigate();
  return (
    <>
      <section className="bg-blue-600 text-white py-16 text-center flex flex-col items-center justify-center">
        <h1 className="text-4xl font-bold mb-4">
          Make Every Workout Memorable
        </h1>
        <p className="text-lg mb-6">
          Automatically generate creative names for your Strava activities
        </p>
        <div className="flex flex-col md:flex-row gap-6 items-center justify-center mb-8">
          <Button
            className="px-12 py-5 text-lg font-bold rounded-lg shadow-lg bg-orange-500 hover:bg-orange-600 transition"
            onClick={() =>
              window.open("https://t.me/strava_snitch_bot", "_blank")
            }
          >
            Try the Bot
          </Button>
          <span className="text-white text-lg font-semibold">or</span>
          <div className="flex flex-col items-center">
            <span className="mb-2 text-base">
              Sign in to manage your activities:
            </span>
            {window.location.hostname === "localhost" ||
            window.location.hostname === "127.0.0.1" ? (
              <Button
                className="px-8 py-3 text-lg font-bold rounded-lg shadow-lg bg-green-600 hover:bg-green-700 transition"
                onClick={async () => {
                  const payload = {
                    user: {
                      id: 1,
                      first_name: "Dev",
                      last_name: "User",
                      username: "devuser",
                    },
                  };
                  await fetch("/tg-auth", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(payload),
                    credentials: "include",
                  });
                  navigate("/user/1");
                }}
              >
                Dev Login (Localhost)
              </Button>
            ) : (
              <TelegramLoginButton
                botName="strava_snitch_bot"
                buttonSize="large"
                dataOnauth={(user: TelegramUser) =>
                  onTelegramAuth(user, navigate)
                }
              />
            )}
          </div>
        </div>
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
            Sometimes I go for a run, just because I'm bored. <br />
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
    </>
  );
};

function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <nav
        style={{
          backgroundColor: "#333",
          padding: "10px 20px",
          color: "white",
        }}
      >
        <ul
          style={{
            listStyleType: "none",
            margin: 0,
            padding: 0,
            display: "flex",
            gap: "20px",
          }}
        >
          <li>
            <Link to="/" style={{ color: "white", textDecoration: "none" }}>
              Home
            </Link>
          </li>
          <li>
            <Link
              to="/activities"
              style={{ color: "white", textDecoration: "none" }}
            >
              Activities
            </Link>
          </li>
        </ul>
      </nav>

      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/activities/:userId" element={<ActivitiesPage />} />
        <Route path="/user/:userId" element={<UserProfilePage />} />
      </Routes>
    </div>
  );
}

export default App;
