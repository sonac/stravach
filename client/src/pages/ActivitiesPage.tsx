import React, { useState, useEffect } from "react";
import { useParams } from "react-router-dom";

interface Activity {
  id: number;
  name: string;
  start_date: string;
  distance: number;
  type: string;
  average_heartrate?: number;
  average_speed?: number;
  moving_time?: number;
  elapsed_time?: number;
  is_updated?: boolean;
  generationStatus?: "idle" | "pending" | "success" | "error";
  generationMessage?: string;
}

const ActivitiesPage: React.FC = () => {
  const { userId } = useParams<{ userId: string }>();
  const [activities, setActivities] = useState<Activity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchActivities = async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const res = await fetch(`/api/activities/${userId}`);
      if (!res.ok) throw new Error("Failed to fetch activities");
      const data = await res.json();
      setActivities(
        data.map((act: any) => ({
          ...act,
          type: act.type || act.activity_type,
          start_date: act.start_date || act.date,
          generationStatus: "idle",
        }))
      );
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const refreshLast10Activities = async () => {
    if (!userId) return;
    setRefreshing(true);
    setError(null);
    try {
      const res = await fetch(`/api/activities-refresh-last-10/${userId}`, {
        method: "POST"
      });
      if (!res.ok) throw new Error("Failed to refresh last 10 activities");
      await fetchActivities();
    } catch (e: any) {
      setError(e.message || "Failed to refresh last 10 activities");
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchActivities();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userId]);

  useEffect(() => {
    if (!userId) return;
    setLoading(true);
    fetch(`/api/activities/${userId}`)
      .then(async (res) => {
        if (!res.ok) throw new Error("Failed to fetch activities");
        const data = await res.json();
        setActivities(
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          data.map((act: any) => ({
            ...act,
            type: act.type || act.activity_type, // fallback for backend field
            start_date: act.start_date || act.date,
            generationStatus: "idle",
          }))
        );
        setLoading(false);
      })
      .catch((e) => {
        setError(e.message);
        setLoading(false);
      });
  }, [userId]);

  const handleGenerateName = async (activityId: number) => {
    setActivities((prevActivities) =>
      prevActivities.map((act) =>
        act.id === activityId
          ? {
              ...act,
              generationStatus: "pending",
              generationMessage: "Sending request...",
            }
          : act
      )
    );

    try {
      const response = await fetch(`/api/activity/${activityId}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
      });

      if (response.ok) {
        setActivities((prevActivities) =>
          prevActivities.map((act) =>
            act.id === activityId
              ? {
                  ...act,
                  generationStatus: "success",
                  generationMessage: "Name generation started!",
                }
              : act
          )
        );
        setTimeout(() => {
          setActivities((prevActivities) =>
            prevActivities.map((act) =>
              act.id === activityId && act.generationStatus === "success"
                ? {
                    ...act,
                    generationStatus: "idle",
                    generationMessage: undefined,
                  }
                : act
            )
          );
        }, 3000);
      } else {
        const errorText = await response.text();
        throw new Error(errorText || "Failed to start name generation.");
      }
    } catch (error) {
      console.error(`Error generating name for activity ${activityId}:`, error);
      setActivities((prevActivities) =>
        prevActivities.map((act) =>
          act.id === activityId
            ? {
                ...act,
                generationStatus: "error",
                generationMessage:
                  error instanceof Error
                    ? error.message
                    : "An unknown error occurred.",
              }
            : act
        )
      );
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64 text-lg">
        Loading activities...
      </div>
    );
  }
  if (error) {
    return (
      <div className="flex justify-center items-center h-64 text-red-600">
        {error}
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-blue-200 py-12 px-4">
      <div className="max-w-3xl mx-auto">
        <h1 className="text-3xl font-bold text-blue-700 mb-8 text-center">
          Your Activities
        </h1>
        <div className="flex justify-end mb-6">
          <button
            onClick={refreshLast10Activities}
            disabled={refreshing}
            className={`py-2 px-6 rounded-lg font-semibold shadow-sm transition duration-200 ${refreshing ? "bg-gray-400 cursor-not-allowed" : "bg-blue-500 hover:bg-blue-600 text-white"}`}
          >
            {refreshing ? "Refreshing..." : "Refresh Last 10"}
          </button>
        </div>
        <div className="grid gap-6 grid-cols-1 md:grid-cols-2">
          {[...activities]
            .sort(
              (a, b) =>
                new Date(b.start_date).getTime() -
                new Date(a.start_date).getTime()
            )
            .map((activity) => (
              <div
                key={activity.id}
                className="bg-white rounded-xl shadow p-6 flex flex-col justify-between border border-blue-100"
              >
                <div>
                  <h3 className="text-xl font-semibold text-blue-800 mb-2">
                    {activity.name}
                  </h3>
                  <p className="text-gray-600 mb-1">
                    <strong>Type:</strong> {activity.type}
                  </p>
                  <p className="text-gray-600 mb-1">
                    <strong>Date:</strong>{" "}
                    {activity.start_date
                      ? new Date(activity.start_date).toLocaleString()
                      : "-"}
                  </p>
                  <p className="text-gray-600 mb-1">
                    <strong>Distance:</strong>{" "}
                    {(activity.distance / 1000).toFixed(2)} km
                  </p>
                  {activity.average_heartrate && (
                    <p className="text-gray-600 mb-1">
                      <strong>Avg HR:</strong> {activity.average_heartrate} bpm
                    </p>
                  )}
                  {activity.average_speed && (
                    <p className="text-gray-600 mb-1">
                      <strong>Avg Speed:</strong>{" "}
                      {(activity.average_speed * 3.6).toFixed(2)} km/h
                    </p>
                  )}
                </div>
                <div className="mt-4">
                  <button
                    onClick={() => handleGenerateName(activity.id)}
                    disabled={activity.generationStatus === "pending"}
                    className={`w-full py-2 px-4 rounded-lg font-semibold transition duration-200 shadow-sm ${
                      activity.generationStatus === "pending"
                        ? "bg-gray-400 cursor-not-allowed"
                        : "bg-blue-600 hover:bg-blue-700 text-white"
                    }`}
                  >
                    {activity.generationStatus === "pending"
                      ? "Generating..."
                      : "Generate Name"}
                  </button>
                  {activity.generationMessage && (
                    <p
                      className={`mt-2 text-center text-sm ${
                        activity.generationStatus === "error"
                          ? "text-red-600"
                          : activity.generationStatus === "success"
                          ? "text-green-600"
                          : "text-gray-600"
                      }`}
                    >
                      {activity.generationMessage}
                    </p>
                  )}
                </div>
              </div>
            ))}
        </div>
      </div>
    </div>
  );
};

export default ActivitiesPage;
