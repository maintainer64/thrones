import { t } from "../i18n/core";
import "./lobby.css";

export function HousePicker({
  houses,
  value,
  onChange,
  disabled = false,
}: {
  houses: { id: string; name: string; available: boolean }[];
  value: string;
  onChange: (house: string) => void;
  disabled?: boolean;
}) {
  return (
    <fieldset className="house-picker" disabled={disabled}>
      <legend>{t("lobby.chooseHouse")}</legend>
      <div className="house-picker-grid">
        {houses.map((house) => (
          <label
            key={house.id}
            className="house-picker-option"
            data-selected={value === house.id}
            data-disabled={!house.available}
          >
            <input
              type="radio"
              name="house-choice"
              value={house.id}
              checked={value === house.id}
              disabled={!house.available}
              onChange={() => onChange(house.id)}
            />
            <img
              src={`/media/crests/${house.id}.svg`}
              width={30}
              height={30}
              alt=""
            />
            <span>
              {house.name}
              {!house.available && <small>{t("lobby.occupied")}</small>}
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}
