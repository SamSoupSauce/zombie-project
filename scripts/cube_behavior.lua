-- ===================================================================
--  Zombie 3D Engine - Lua Scripting Demo
--  File: scripts/cube_behavior.lua
--  Tip: Edit this file while the game is running to see instant Hot-Reload!
-- ===================================================================

local time_accum = 0
local mode_timer = 0
local modes = {"faces", "gradient", "chroma"}
local current_mode_index = 1
local pulse_enabled = true -- Toggle breathing pulsation around base size

-- init() is called once when the script loads or hot-reloads
function init()
    engine.log("Lua script loaded! Initializing cube behavior.")
    
    cube.set_speed(35, 55, 25)
    cube.set_wireframe(true)
    cube.set_mode("gradient")
    
    time_accum = 0
    mode_timer = 0
    engine.log("Init complete. Cube set to Gradient mode.")
end

-- on_reload() is called when you save edits to this file
function on_reload()
    engine.log("HOT-RELOAD: Script refreshed without restarting!")
end

-- update(dt) is called every frame (dt is delta time in seconds)
function update(dt)
    time_accum = time_accum + dt
    mode_timer = mode_timer + dt

    -- 1. Custom Scripted Rotation:
    local speed_y = 65.0
    local speed_x = 40.0
    local speed_z = 25.0
    cube.rotate(speed_x * dt, speed_y * dt, speed_z * dt)

    -- 2. Breathing / Pulsing Scale Effect:
    -- Modulates scale around the cube's base size (set via cube.set_size(num))
    if pulse_enabled then
        local pulse_speed = 3.0
        local pulse_amount = 0.08
        local dynamic_scale = 1.0 + (math.sin(time_accum * pulse_speed) * pulse_amount)
        cube.set_scale(dynamic_scale)
    end

    -- 4. Dynamic Mode Switching:
    if mode_timer >= 7.0 then
        mode_timer = 0
        current_mode_index = (current_mode_index % #modes) + 1
        local new_mode = modes[current_mode_index]
        cube.set_mode(new_mode)
        engine.log("Lua automatically transitioned mode to: " .. new_mode)
    end

    -- 5. Scripted Input Event Check:
    if input.is_key_pressed("R") then
        mode_timer = 0
        current_mode_index = (current_mode_index % #modes) + 1
        cube.set_mode(modes[current_mode_index])
        engine.log("User pressed R: Mode changed to " .. modes[current_mode_index])
    end
end
