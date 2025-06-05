import React, { useState, useEffect } from 'react';

interface Activity {
  id: string;
  name: string;
  date: string;
  distance: number;
  type: string;
  generatedName?: string;
  generationStatus?: 'idle' | 'pending' | 'success' | 'error';
  generationMessage?: string;
}

const mockActivities: Activity[] = [
  {
    id: '1',
    name: 'Morning Run',
    date: '2025-05-24',
    distance: 5.2,
    type: 'Run',
    generationStatus: 'idle',
  },
  {
    id: '2',
    name: 'Evening Bike Ride',
    date: '2025-05-23',
    distance: 25.0,
    type: 'Ride',
    generationStatus: 'idle',
  },
  {
    id: '3',
    name: 'Lunchtime Swim',
    date: '2025-05-22',
    distance: 1.5,
    type: 'Swim',
    generationStatus: 'idle',
  },
];

const ActivitiesPage: React.FC = () => {
  const [activities, setActivities] = useState<Activity[]>([]);

  useEffect(() => {
    setActivities(mockActivities.map(act => ({ ...act, generationStatus: 'idle' })));
  }, []);

  const handleGenerateName = async (activityId: string) => {
    setActivities(prevActivities =>
      prevActivities.map(act =>
        act.id === activityId
          ? { ...act, generationStatus: 'pending', generationMessage: 'Sending request...' }
          : act
      )
    );

    try {
      const response = await fetch(`/activity/${activityId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (response.ok) {
        setActivities(prevActivities =>
          prevActivities.map(act =>
            act.id === activityId
              ? { ...act, generationStatus: 'success', generationMessage: 'Name generation started!' }
              : act
          )
        );
        setTimeout(() => {
          setActivities(prevActivities =>
            prevActivities.map(act =>
              act.id === activityId && act.generationStatus === 'success'
                ? { ...act, generationStatus: 'idle', generationMessage: undefined }
                : act
            )
          );
        }, 3000);
      } else {
        const errorText = await response.text();
        throw new Error(errorText || 'Failed to start name generation.');
      }
    } catch (error) {
      console.error(`Error generating name for activity ${activityId}:`, error);
      setActivities(prevActivities =>
        prevActivities.map(act =>
          act.id === activityId
            ? { ...act, generationStatus: 'error', generationMessage: error instanceof Error ? error.message : 'An unknown error occurred.' }
            : act
        )
      );
    }
  };

  if (!activities.length) {
    return <div>Loading activities...</div>;
  }

  return (
    <div style={{ padding: '20px', fontFamily: 'Arial, sans-serif' }}>
      <h1>Your Activities</h1>
      <ul style={{ listStyleType: 'none', padding: 0 }}>
        {activities.map(activity => (
          <li key={activity.id} style={{
            border: '1px solid #ddd',
            marginBottom: '10px',
            padding: '15px',
            borderRadius: '5px',
            backgroundColor: '#f9f9f9'
          }}>
            <h3 style={{ marginTop: 0 }}>{activity.name}</h3>
            <p><strong>Type:</strong> {activity.type}</p>
            <p><strong>Date:</strong> {activity.date}</p>
            <p><strong>Distance:</strong> {activity.distance} km</p>
            {activity.generatedName && (
              <p style={{ color: 'green' }}>
                <strong>Suggested Name:</strong> {activity.generatedName}
              </p>
            )}
            <button 
              onClick={() => handleGenerateName(activity.id)}
              disabled={activity.generationStatus === 'pending'}
              style={{
                padding: '8px 15px',
                backgroundColor: activity.generationStatus === 'pending' ? '#ccc' : '#007bff',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: activity.generationStatus === 'pending' ? 'not-allowed' : 'pointer',
                marginTop: '10px'
              }}
            >
              {activity.generationStatus === 'pending' ? 'Generating...' : 'Generate Name'}
            </button>
            {activity.generationMessage && (
              <p style={{
                marginTop: '10px',
                color: activity.generationStatus === 'error' ? 'red' : (activity.generationStatus === 'success' ? 'green' : '#555'),
                fontSize: '0.9em'
              }}>
                {activity.generationMessage}
              </p>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
};

export default ActivitiesPage;
